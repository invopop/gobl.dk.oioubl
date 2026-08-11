package oioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

// applyLines reworks the base's lines for OIOUBL: a gross line amount with the
// allowances moved up to document level, and OIOUBL's own tax category ID.
func (ui *Invoice) applyLines(inv *bill.Invoice) {
	lines := ui.InvoiceLines
	if len(ui.CreditNoteLines) > 0 {
		lines = ui.CreditNoteLines
	}
	for i := range lines {
		if i >= len(inv.Lines) {
			break
		}
		applyLine(&lines[i], inv.Lines[i])
	}
	applyLineTaxCategories(ui.InvoiceLines)
	applyLineTaxCategories(ui.CreditNoteLines)
}

func applyLine(invLine *ubl.InvoiceLine, l *bill.Line) {
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
		invLine.Item.ClassifiedTaxCategory.ID = &ubl.IDType{Value: cat}
	}
}

// applyLineTaxCategories stamps the tax categories on each line's classified
// category, line-level subtotals, and promoted allowance/charges.
func applyLineTaxCategories(lines []ubl.InvoiceLine) {
	for i := range lines {
		line := &lines[i]
		if line.Item != nil && line.Item.ClassifiedTaxCategory != nil {
			applyClassifiedTaxCategory(line.Item.ClassifiedTaxCategory)
		}
		for j := range line.TaxTotal {
			for k := range line.TaxTotal[j].TaxSubtotal {
				st := &line.TaxTotal[j].TaxSubtotal[k]
				// Skip excise subtotals -- the VAT overlay here would overwrite their own scheme/TaxTypeCode.
				if st.TaxCategory.ID != nil && st.TaxCategory.ID.Value == taxCategoryExcise {
					continue
				}
				applyTaxCategory(&st.TaxCategory)
			}
		}
	}
}

// makeLineTaxTotals builds the OIOUBL line-level cac:TaxTotal, required on
// every line, even at 0% (F-INV138 / F-LIB404).
func makeLineTaxTotals(line *bill.Line, ccy string) []ubl.TaxTotal {
	if line == nil || len(line.Taxes) == 0 {
		return nil
	}

	// Gross line amount (Price×Qty), what OIOUBL wants per F-LIB402; nil means an unpriced line, nothing to tax.
	if line.Sum == nil {
		return nil
	}
	taxable := *line.Sum

	// VAT is charged on the amount including excise duty (F-LIB402).
	for _, ch := range line.Charges {
		if chargeIsExcise(ch.Key) {
			taxable = taxable.Add(rescaleToCurrency(ch.Amount, ccy))
		}
	}

	taxTotal := ubl.TaxTotal{
		TaxAmount: ubl.Amount{Value: "0", CurrencyID: &ccy},
	}
	totalAmount := num.MakeAmount(0, taxable.Exp())

	for _, t := range line.Taxes {
		subtotal := ubl.TaxSubtotal{
			TaxableAmount: ubl.Amount{Value: taxable.String(), CurrencyID: &ccy},
		}
		taxCat := ubl.TaxCategory{}

		if k := taxCategoryID(t.Key); k != "" {
			taxCat.ID = &ubl.IDType{Value: k}
		}

		if t.Percent != nil {
			taxCat.Percent = ptr(t.Percent.StringWithoutSymbol())
			amount := t.Percent.Of(taxable).Rescale(taxable.Exp())
			subtotal.TaxAmount = ubl.Amount{Value: amount.String(), CurrencyID: &ccy}
			totalAmount = totalAmount.Add(amount)
		} else {
			// Exempt lines still need a currency-precision zero ("0.00"); F-LIB263 rejects a bare "0".
			subtotal.TaxAmount = ubl.Amount{Value: num.MakeAmount(0, taxable.Exp()).String(), CurrencyID: &ccy}
		}

		if t.Category != "" {
			taxCat.TaxScheme = &ubl.TaxScheme{ID: ubl.IDType{Value: t.Category.String()}}
		}
		subtotal.TaxCategory = taxCat
		taxTotal.TaxSubtotal = append(taxTotal.TaxSubtotal, subtotal)
	}

	taxTotal.TaxAmount = ubl.Amount{Value: totalAmount.String(), CurrencyID: &ccy}

	// Also emit line-level cac:TaxTotal/Excise blocks so the wire records which line each duty belongs to.
	totals := []ubl.TaxTotal{taxTotal}
	exciseTotals, _ := makeExciseTaxTotals(collectLineExcise(line, ccy), ccy)
	totals = append(totals, exciseTotals...)
	return totals
}

// makeLineCharges rebuilds a line's allowances/charges for document-level promotion,
// with OIOUBL's required TaxCategory (F-LIB226) and decimal-factor MultiplierFactorNumeric (F-LIB228).
func makeLineCharges(charges []*bill.LineCharge, discounts []*bill.LineDiscount, ccy string, baseSum *num.Amount, taxes tax.Set) []*ubl.AllowanceCharge {
	var acs []*ubl.AllowanceCharge
	for _, ch := range charges {
		ac := &ubl.AllowanceCharge{
			ChargeIndicator: true,
			Amount:          ubl.Amount{Value: rescaleToCurrency(ch.Amount, ccy).String(), CurrencyID: &ccy},
		}
		if e := ch.Ext.Get(untdid.ExtKeyCharge).String(); e != "" {
			ac.AllowanceChargeReasonCode = ptr(e)
		}
		if ch.Reason != "" {
			ac.AllowanceChargeReason = ptr(ch.Reason)
		}
		ac.BaseAmount = chargeBaseAmount(ch.Percent, ch.Base, baseSum, ccy)
		applyLineAllowanceCharge(ac, ch.Percent, taxes)
		acs = append(acs, ac)
	}
	for _, d := range discounts {
		ac := &ubl.AllowanceCharge{
			Amount: ubl.Amount{Value: rescaleToCurrency(d.Amount, ccy).String(), CurrencyID: &ccy},
		}
		if e := d.Ext.Get(untdid.ExtKeyAllowance).String(); e != "" {
			ac.AllowanceChargeReasonCode = ptr(e)
		}
		if d.Reason != "" {
			ac.AllowanceChargeReason = ptr(d.Reason)
		}
		ac.BaseAmount = chargeBaseAmount(d.Percent, d.Base, baseSum, ccy)
		applyLineAllowanceCharge(ac, d.Percent, taxes)
		acs = append(acs, ac)
	}
	return acs
}

// applyLineAllowanceCharge stamps MultiplierFactorNumeric/TaxCategory.
func applyLineAllowanceCharge(ac *ubl.AllowanceCharge, pct *num.Percentage, taxes tax.Set) {
	if pct != nil {
		ac.MultiplierFactorNumeric = ptr(allowanceMultiplier(pct))
	}
	ac.TaxCategory = makeTaxCategory(taxes)
}

// rescaleToCurrency rounds the amount to the natural precision of the given
// currency code, falling back to the amount's own precision if unknown.
func rescaleToCurrency(a num.Amount, ccy string) num.Amount {
	if def := currency.Code(ccy).Def(); def != nil {
		return def.Rescale(a)
	}
	return a
}
