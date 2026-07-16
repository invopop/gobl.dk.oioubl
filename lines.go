package dkoioubl

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

// decorateLines adjusts the base's already-built InvoiceLines/CreditNoteLines
// for OIOUBL: gross line amount with no line-level allowances (promoted to
// the document instead, F-INV126/128/129), no forbidden OriginCountry
// (F-INV211/F-CRN109), and the OIOUBL ClassifiedTaxCategory ID (the base
// reads it from the UNTDID tax-category ext, which our normalizer strips).
func (ui *Invoice) decorateLines(inv *bill.Invoice) {
	decorateLineSet(ui.InvoiceLines, inv.Lines)
	decorateLineSet(ui.CreditNoteLines, inv.Lines)
	applyLineTaxCategories(ui.InvoiceLines)
	applyLineTaxCategories(ui.CreditNoteLines)
}

func decorateLineSet(lines []InvoiceLine, glines []*bill.Line) {
	for i := range lines {
		if i >= len(glines) {
			break
		}
		decorateLine(&lines[i], glines[i])
	}
}

func decorateLine(invLine *InvoiceLine, l *bill.Line) {
	invLine.AllowanceCharge = nil
	ccy := ""
	if invLine.LineExtensionAmount.CurrencyID != nil {
		ccy = *invLine.LineExtensionAmount.CurrencyID
	}
	// F-INV348: gross Price×Qty here; line allowances net at the document level.
	if l.Sum != nil {
		invLine.LineExtensionAmount.Value = l.Sum.String()
	}
	invLine.TaxTotal = makeLineTaxTotals(l, ccy)
	if invLine.Item == nil {
		return
	}
	invLine.Item.OriginCountry = nil
	if invLine.Item.ClassifiedTaxCategory == nil || len(l.Taxes) == 0 {
		return
	}
	if cat := taxCategoryID(l.Taxes[0].Key); cat != "" {
		invLine.Item.ClassifiedTaxCategory.ID = &IDType{Value: cat}
	}
}

// applyLineTaxCategories stamps the tax categories on each line's classified
// category, line-level subtotals, and promoted allowance/charges.
func applyLineTaxCategories(lines []InvoiceLine) {
	for i := range lines {
		line := &lines[i]
		if line.Item != nil && line.Item.ClassifiedTaxCategory != nil {
			applyClassifiedTaxCategory(line.Item.ClassifiedTaxCategory)
		}
		for j := range line.TaxTotal {
			for k := range line.TaxTotal[j].TaxSubtotal {
				st := &line.TaxTotal[j].TaxSubtotal[k]
				// Excise subtotals carry their own scheme code, name and TaxTypeCode;
				// the VAT overlay would clobber them with 63/Moms, so leave them be.
				if st.TaxCategory.ID != nil && st.TaxCategory.ID.Value == taxCategoryExcise {
					continue
				}
				applyTaxCategory(&st.TaxCategory)
			}
		}
		for _, ac := range line.AllowanceCharge {
			for _, tc := range ac.TaxCategory {
				applyTaxCategory(tc)
			}
		}
	}
}

// rescaleToCurrency rounds the amount to the currency's natural precision
// (2 for EUR, 0 for JPY), or leaves it unchanged for an unknown currency.
func rescaleToCurrency(a num.Amount, ccy string) num.Amount {
	if def := currency.Code(ccy).Def(); def != nil {
		return def.Rescale(a)
	}
	return a
}

// makeLineTaxTotals builds the OIOUBL line-level cac:TaxTotal, required on
// every line, even at 0% (F-INV138 / F-LIB404).
func makeLineTaxTotals(line *bill.Line, ccy string) []TaxTotal {
	if line == nil || len(line.Taxes) == 0 {
		return nil
	}

	var taxable num.Amount
	switch {
	case line.Sum != nil:
		// Line TaxableAmount is gross (Price×Qty); the discount is taken once at document level (F-LIB402).
		taxable = *line.Sum
	case line.Total != nil:
		taxable = *line.Total
	default:
		return nil
	}

	// An excise duty is emitted as its own tax, not an AllowanceCharge, so fold
	// it into the VAT taxable base here: VAT lands on the duty-inclusive amount (F-LIB402).
	for _, ch := range line.Charges {
		if chargeIsExcise(ch.Key) {
			taxable = taxable.Add(rescaleToCurrency(ch.Amount, ccy))
		}
	}

	taxTotal := TaxTotal{
		TaxAmount: Amount{Value: "0", CurrencyID: &ccy},
	}
	totalAmount := num.MakeAmount(0, taxable.Exp())

	for _, t := range line.Taxes {
		subtotal := TaxSubtotal{
			TaxableAmount: Amount{Value: taxable.String(), CurrencyID: &ccy},
		}
		taxCat := TaxCategory{}

		if k := taxCategoryID(t.Key); k != "" {
			taxCat.ID = &IDType{Value: k}
		}

		if t.Percent != nil {
			p := t.Percent.StringWithoutSymbol()
			taxCat.Percent = &p
			amount := t.Percent.Of(taxable).Rescale(taxable.Exp())
			subtotal.TaxAmount = Amount{Value: amount.String(), CurrencyID: &ccy}
			totalAmount = totalAmount.Add(amount)
		} else {
			// No percent (e.g. exempt): still emit at currency precision
			// ("0.00"), or OIOUBL F-LIB263 rejects a bare "0".
			subtotal.TaxAmount = Amount{Value: num.MakeAmount(0, taxable.Exp()).String(), CurrencyID: &ccy}
		}

		if t.Category != "" {
			taxCat.TaxScheme = &TaxScheme{ID: IDType{Value: t.Category.String()}}
		}
		subtotal.TaxCategory = taxCat
		taxTotal.TaxSubtotal = append(taxTotal.TaxSubtotal, subtotal)
	}

	taxTotal.TaxAmount = Amount{Value: totalAmount.String(), CurrencyID: &ccy}

	// Also emit line-level cac:TaxTotal/Excise blocks so the wire records which line each duty belongs to.
	totals := []TaxTotal{taxTotal}
	totals = append(totals, makeExciseTaxTotals(collectLineExcise(line, ccy), ccy)...)
	return totals
}

// OIOUBL: stamps a TaxCategory on each line allowance/charge (F-LIB226).
func makeLineCharges(charges []*bill.LineCharge, discounts []*bill.LineDiscount, ccy string, baseSum *num.Amount, taxes tax.Set) []*AllowanceCharge {
	var allowanceCharges []*AllowanceCharge
	// BR-DEC-24 / UBL-DT-01: GOBL only clamps line charge/discount amounts to
	// the item price's precision, which can exceed the currency's, so they are
	// rescaled here; the base (the line sum) is already at currency precision.
	var base *Amount
	if baseSum != nil {
		base = &Amount{
			Value:      baseSum.String(),
			CurrencyID: &ccy,
		}
	}
	for _, ch := range charges {
		ac := &AllowanceCharge{
			ChargeIndicator: true,
			Amount: Amount{
				Value:      rescaleToCurrency(ch.Amount, ccy).String(),
				CurrencyID: &ccy,
			},
		}
		if e := ch.Ext.Get(untdid.ExtKeyCharge).String(); e != "" {
			ac.AllowanceChargeReasonCode = &e
		}
		if ch.Reason != "" {
			ac.AllowanceChargeReason = &ch.Reason
		}
		if ch.Percent != nil {
			p := allowanceMultiplier(ch.Percent)
			ac.MultiplierFactorNumeric = &p
			if base != nil {
				ac.BaseAmount = base
			}
		}
		ac.TaxCategory = makeTaxCategory(taxes) // F-LIB226: line allowance needs a TaxCategory
		allowanceCharges = append(allowanceCharges, ac)
	}
	for _, d := range discounts {
		ac := &AllowanceCharge{
			ChargeIndicator: false,
			Amount: Amount{
				Value:      rescaleToCurrency(d.Amount, ccy).String(),
				CurrencyID: &ccy,
			},
		}
		if e := d.Ext.Get(untdid.ExtKeyAllowance).String(); e != "" {
			ac.AllowanceChargeReasonCode = &e
		}
		if d.Reason != "" {
			ac.AllowanceChargeReason = &d.Reason
		}
		if d.Percent != nil {
			p := allowanceMultiplier(d.Percent)
			ac.MultiplierFactorNumeric = &p
			if base != nil {
				ac.BaseAmount = base
			}
		}
		ac.TaxCategory = makeTaxCategory(taxes) // F-LIB226: line allowance needs a TaxCategory
		allowanceCharges = append(allowanceCharges, ac)
	}
	return allowanceCharges
}
