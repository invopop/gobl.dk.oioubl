package dkoioubl

import (
	"strconv"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/num"
)

// addMonetaryTotal rebuilds LegalMonetaryTotal for OIOUBL's gross line amounts
// (F-INV348), folding line allowances/charges into the document totals.
func (ui *Invoice) addMonetaryTotal(inv *bill.Invoice, currency string) {
	t := inv.Totals
	exp := t.Sum.Exp()
	grossSum := num.MakeAmount(0, exp)
	lineDiscounts := num.MakeAmount(0, exp)
	lineCharges := num.MakeAmount(0, exp)
	// excise holds duty charges emitted as cac:TaxTotal/Excise subtotals; they
	// land in TaxInclusiveAmount as tax rather than in ChargeTotalAmount.
	excise := num.MakeAmount(0, exp)
	for _, l := range inv.Lines {
		if l.Sum != nil {
			grossSum = grossSum.Add(*l.Sum)
		}
		for _, d := range l.Discounts {
			lineDiscounts = lineDiscounts.Add(d.Amount)
		}
		ordinary := make([]*bill.LineCharge, 0, len(l.Charges))
		for _, c := range l.Charges {
			if chargeIsExcise(c.Key) {
				excise = excise.Add(rescaleToCurrency(c.Amount, currency))
				continue
			}
			lineCharges = lineCharges.Add(c.Amount)
			ordinary = append(ordinary, c)
		}
		// Promote ordinary line allowances/charges to document level (F-INV129/F-INV130).
		for _, ac := range makeLineCharges(ordinary, l.Discounts, currency, l.Sum, l.Taxes) {
			ui.AllowanceCharge = append(ui.AllowanceCharge, *ac)
		}
	}
	ui.LegalMonetaryTotal.LineExtensionAmount = Amount{Value: grossSum.String(), CurrencyID: &currency}
	allow := lineDiscounts
	if t.Discount != nil {
		allow = allow.Add(*t.Discount)
	}
	if !allow.IsZero() {
		ui.LegalMonetaryTotal.AllowanceTotalAmount = &Amount{Value: allow.String(), CurrencyID: &currency}
	}
	chg := lineCharges
	if t.Charge != nil {
		chg = chg.Add(*t.Charge)
	}
	for _, ch := range inv.Charges {
		if chargeIsExcise(ch.Key) {
			excise = excise.Add(ch.Amount)
			chg = chg.Subtract(ch.Amount) // counted in t.Charge above; OIOUBL emits it as tax
		}
	}
	// addTotals pre-sets ChargeTotalAmount from t.Charge, which also includes
	// any excise duty; clear it back out here if excise absorbed all of it, or
	// F-INV130/F-INV128/F-INV133 see a stale charge with no matching
	// AllowanceCharge (an excise duty is never promoted to one).
	if chg.IsZero() {
		ui.LegalMonetaryTotal.ChargeTotalAmount = nil
	} else {
		ui.LegalMonetaryTotal.ChargeTotalAmount = &Amount{Value: chg.String(), CurrencyID: &currency}
	}
	// OIOUBL rounds per line then sums (F-INV128/F-INV133), which can differ from
	// GOBL's end-rounding by a cent; recompute totals from the rounded components.
	incl := grossSum.Add(t.Tax).Add(excise).Add(chg).Subtract(allow)
	if t.Rounding != nil {
		incl = incl.Add(*t.Rounding)
	}
	ui.LegalMonetaryTotal.TaxInclusiveAmount = Amount{Value: incl.String(), CurrencyID: &currency}
	pay := incl
	if t.Advances != nil {
		pay = pay.Subtract(*t.Advances)
	}
	ui.LegalMonetaryTotal.PayableAmount = &Amount{Value: pay.String(), CurrencyID: &currency}
}

// addPrepaidPayments emits a cac:PrepaidPayment per GOBL advance; their
// PaidAmounts must sum to LegalMonetaryTotal/PrepaidAmount (F-INV131).
func (ui *Invoice) addPrepaidPayments(inv *bill.Invoice, currency string) {
	if inv.Payment == nil {
		return
	}
	for i, adv := range inv.Payment.Advances {
		if adv == nil {
			continue
		}
		pp := PrepaidPayment{
			ID:         strconv.Itoa(i + 1),
			PaidAmount: &Amount{Value: adv.Amount.String(), CurrencyID: &currency},
		}
		if adv.Date != nil {
			d := ubl.FormatDate(*adv.Date)
			pp.ReceivedDate = &d
		}
		if adv.Ref != "" {
			ref := adv.Ref
			pp.InstructionID = &ref
		}
		ui.PrepaidPayment = append(ui.PrepaidPayment, pp)
	}
}

func (ui *Invoice) addTotals(inv *bill.Invoice) {
	if inv == nil || inv.Totals == nil {
		return
	}
	t := inv.Totals
	currency := inv.Currency.String()

	// LineExtensionAmount/TaxExclusiveAmount/TaxInclusiveAmount/PayableAmount/
	// ChargeTotalAmount are all unconditionally overwritten below (addMonetaryTotal,
	// applyTotals), so there's no need to set them from t here first.
	ui.addMonetaryTotal(inv, currency)
	ui.addPrepaidPayments(inv, currency)

	if t.Rounding != nil {
		ui.LegalMonetaryTotal.PayableRoundingAmount = &Amount{Value: t.Rounding.String(), CurrencyID: &currency}
	}
	if t.Advances != nil {
		ui.LegalMonetaryTotal.PrepaidAmount = &Amount{Value: t.Advances.String(), CurrencyID: &currency}
	}

	ui.TaxTotal = []TaxTotal{
		{
			TaxAmount: Amount{Value: t.Tax.String(), CurrencyID: &currency},
		},
	}
	ui.addVATSubtotals(inv, currency)

	// Non-VAT excise duties travel as their own cac:TaxTotal blocks (the VAT total
	// already includes them in its base); applyTotals sums them into TaxExclusiveAmount.
	ui.TaxTotal = append(ui.TaxTotal, makeExciseTaxTotals(collectExcise(inv, currency), currency)...)
}

// applyTotals stamps taxcategoryid attributes and re-interprets TaxExclusiveAmount as the total tax (F-INV127).
func (ui *Invoice) applyTotals() {
	for i := range ui.TaxTotal {
		for j := range ui.TaxTotal[i].TaxSubtotal {
			st := &ui.TaxTotal[i].TaxSubtotal[j]
			// Excise subtotals carry their own scheme code, name and TaxTypeCode; the
			// VAT overlay would clobber them with 63/Moms, so leave them untouched.
			if st.TaxCategory.ID != nil && st.TaxCategory.ID.Value == taxCategoryExcise {
				continue
			}
			applyTaxCategory(&st.TaxCategory)
		}
	}
	for i := range ui.AllowanceCharge {
		for _, tc := range ui.AllowanceCharge[i].TaxCategory {
			applyTaxCategory(tc)
		}
	}
	// TaxExclusiveAmount is the sum of all tax — VAT plus any excise (F-INV127).
	ui.LegalMonetaryTotal.TaxExclusiveAmount = sumTaxTotalAmounts(ui.TaxTotal)
}

// sumTaxTotalAmounts totals the TaxAmount of every cac:TaxTotal (VAT plus excise).
func sumTaxTotalAmounts(totals []TaxTotal) Amount {
	if len(totals) == 0 {
		return Amount{}
	}
	if len(totals) == 1 {
		return totals[0].TaxAmount
	}
	sum, err := num.AmountFromString(ubl.NormalizeNumericString(totals[0].TaxAmount.Value))
	if err != nil {
		return totals[0].TaxAmount
	}
	for _, tt := range totals[1:] {
		a, err := num.AmountFromString(ubl.NormalizeNumericString(tt.TaxAmount.Value))
		if err != nil {
			continue
		}
		sum = sum.Add(a.Rescale(sum.Exp()))
	}
	return Amount{Value: sum.String(), CurrencyID: totals[0].TaxAmount.CurrencyID}
}
