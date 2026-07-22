package dkoioubl

import (
	"strconv"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/cef"
	"github.com/invopop/gobl/cbc"
	cur "github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

// applyLineCategoryAndTaxTotalFlavor rebuilds lines, tax categories, charges
// and tax/monetary totals to OIOUBL's conventions. Totals have no reusable
// equivalent in the base (excise-as-tax, document-level promotion), so they're
// rebuilt outright.
func (ui *Invoice) applyLineCategoryAndTaxTotalFlavor(inv *bill.Invoice) error {
	// Drop the tax currency unless a StandardRated rate (%>0) carries it (F-LIB373/F-INV018).
	if ui.TaxCurrencyCode != "" && !hasStandardRated(inv) {
		ui.TaxCurrencyCode = ""
	}
	ui.applyCharges(inv)
	ui.TaxTotal = nil
	ui.addTotals(inv)
	ui.applyLines(inv)
	ui.applyTotals()
	return nil
}

// addMonetaryTotal rebuilds LegalMonetaryTotal with gross line amounts (F-INV348).
func (ui *Invoice) addMonetaryTotal(inv *bill.Invoice, currency string) {
	t := inv.Totals
	exp := t.Sum.Exp()
	grossSum := num.MakeAmount(0, exp)
	lineDiscounts := num.MakeAmount(0, exp)
	lineCharges := num.MakeAmount(0, exp)
	// excise duties land in TaxInclusiveAmount as tax, not in ChargeTotalAmount.
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
	ui.LegalMonetaryTotal.LineExtensionAmount = ubl.Amount{Value: grossSum.String(), CurrencyID: &currency}
	allow := lineDiscounts
	if t.Discount != nil {
		allow = allow.Add(*t.Discount)
	}
	if !allow.IsZero() {
		ui.LegalMonetaryTotal.AllowanceTotalAmount = &ubl.Amount{Value: allow.String(), CurrencyID: &currency}
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
	// Clear a charge total left over purely from excise, which is never
	// promoted to an AllowanceCharge (F-INV128/F-INV130/F-INV133).
	if chg.IsZero() {
		ui.LegalMonetaryTotal.ChargeTotalAmount = nil
	} else {
		ui.LegalMonetaryTotal.ChargeTotalAmount = &ubl.Amount{Value: chg.String(), CurrencyID: &currency}
	}
	// OIOUBL rounds per line then sums (F-INV128/F-INV133), which can differ from
	// GOBL's end-rounding by a cent; recompute totals from the rounded components.
	incl := grossSum.Add(t.Tax).Add(excise).Add(chg).Subtract(allow)
	if t.Rounding != nil {
		incl = incl.Add(*t.Rounding)
	}
	ui.LegalMonetaryTotal.TaxInclusiveAmount = ubl.Amount{Value: incl.String(), CurrencyID: &currency}
	pay := incl
	if t.Advances != nil {
		pay = pay.Subtract(*t.Advances)
	}
	ui.LegalMonetaryTotal.PayableAmount = &ubl.Amount{Value: pay.String(), CurrencyID: &currency}
}

// addPrepaidPayments emits a cac:PrepaidPayment per GOBL advance (F-INV131).
func (ui *Invoice) addPrepaidPayments(inv *bill.Invoice, currency string) {
	if inv.Payment == nil {
		return
	}
	for i, adv := range inv.Payment.Advances {
		if adv == nil {
			continue
		}
		pp := ubl.PrepaidPayment{
			ID:         strconv.Itoa(i + 1),
			PaidAmount: &ubl.Amount{Value: adv.Amount.String(), CurrencyID: &currency},
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

	ui.addMonetaryTotal(inv, currency)
	ui.addPrepaidPayments(inv, currency)

	if t.Rounding != nil {
		ui.LegalMonetaryTotal.PayableRoundingAmount = &ubl.Amount{Value: t.Rounding.String(), CurrencyID: &currency}
	}
	if t.Advances != nil {
		ui.LegalMonetaryTotal.PrepaidAmount = &ubl.Amount{Value: t.Advances.String(), CurrencyID: &currency}
	}

	ui.TaxTotal = []ubl.TaxTotal{
		{
			TaxAmount: ubl.Amount{Value: t.Tax.String(), CurrencyID: &currency},
		},
	}
	ui.addVATSubtotals(inv, currency)

	// Non-VAT excise duties travel as their own cac:TaxTotal blocks (the VAT total
	// already includes them in its base); applyTotals sums them into TaxExclusiveAmount.
	ui.TaxTotal = append(ui.TaxTotal, makeExciseTaxTotals(collectExcise(inv, currency), currency)...)
}

// addVATSubtotals builds one cac:TaxSubtotal per VAT rate row onto ui.TaxTotal[0].
func (ui *Invoice) addVATSubtotals(inv *bill.Invoice, currency string) {
	t := inv.Totals
	if t.Taxes == nil || len(t.Taxes.Categories) == 0 {
		return
	}
	rCurrency := inv.RegimeDef().Currency.String()
	var accRate *cur.ExchangeRate
	if inv.Currency != inv.RegimeDef().Currency {
		accRate = cur.MatchExchangeRate(inv.ExchangeRates, inv.Currency, inv.RegimeDef().Currency)
	}
	exciseBases := exciseVATBases(inv)
	for _, cat := range t.Taxes.Categories {
		foldedBase, hasRealRate := foldedBaseByCategory(cat.Rates)
		for _, r := range cat.Rates {
			catID := taxCategoryID(r.Key)
			if r.Percent == nil && r.Amount.IsZero() && hasRealRate[catID] {
				// Folded into the category's real rate row below instead of
				// its own (G17 3.1: a discount/charge's VAT-liability flag).
				// A category with no real rate of its own (e.g. ReverseCharge,
				// itself percent-less and zero-amount) keeps its own row.
				continue
			}
			if isExciseOnlyRate(cat, r, exciseBases) {
				continue
			}
			base := r.Base
			if extra, ok := foldedBase[catID]; ok {
				base = base.Add(extra.Rescale(base.Exp()))
			}
			subtotal := buildVATSubtotal(inv, cat, r, base, accRate, rCurrency, currency)
			ui.TaxTotal[0].TaxSubtotal = append(ui.TaxTotal[0].TaxSubtotal, subtotal)
		}
	}
}

// foldedBaseByCategory sums zero-amount, percent-less rate rows (discount/charge
// VAT-liability flags, not real rates -- G17 3.1) into the matching real-rate
// category's base, since OIOUBL forbids splitting one category across two
// TaxSubtotal entries (G27 3.5); it also reports which categories have a real row.
func foldedBaseByCategory(rates []*tax.RateTotal) (map[string]num.Amount, map[string]bool) {
	hasRealRate := make(map[string]bool)
	for _, r := range rates {
		if r.Percent == nil && r.Amount.IsZero() {
			continue
		}
		if catID := taxCategoryID(r.Key); catID != "" {
			hasRealRate[catID] = true
		}
	}
	folded := make(map[string]num.Amount)
	for _, r := range rates {
		if r.Percent != nil || !r.Amount.IsZero() {
			continue
		}
		catID := taxCategoryID(r.Key)
		if catID == "" || !hasRealRate[catID] {
			continue
		}
		if sum, ok := folded[catID]; ok {
			folded[catID] = sum.Add(r.Base.Rescale(sum.Exp()))
		} else {
			folded[catID] = r.Base
		}
	}
	return folded, hasRealRate
}

// isExciseOnlyRate reports whether a VAT rate row is owed entirely to excise, so it gets no subtotal of its own (F-LIB404).
func isExciseOnlyRate(cat *tax.CategoryTotal, r *tax.RateTotal, exciseBases map[cbc.Key]num.Amount) bool {
	if cat.Code != tax.CategoryVAT || !r.Amount.IsZero() {
		return false
	}
	base, ok := exciseBases[r.Key]
	return ok && r.Base.Compare(base.Rescale(r.Base.Exp())) == 0
}

// buildVATSubtotal builds one cac:TaxSubtotal for a VAT rate row; percent is
// required unless the category is "O" (outside scope). Copied from gobl.ubl's
// addTotals subtotal loop (totals.go) since that TaxTotal is discarded
// wholesale -- deltas: category ID from the GOBL VAT key (taxCategoryID)
// instead of the UNTDID ext our normalizer strips, TransactionCurrencyTaxAmount
// (F-LIB373), and excise-only-rate skipping. Re-diff against gobl.ubl on every
// version bump.
func buildVATSubtotal(inv *bill.Invoice, cat *tax.CategoryTotal, r *tax.RateTotal, base num.Amount, accRate *cur.ExchangeRate, rCurrency, currency string) ubl.TaxSubtotal {
	subtotal := ubl.TaxSubtotal{
		TaxAmount: ubl.Amount{Value: r.Amount.String(), CurrencyID: &currency},
	}
	if base != (num.Amount{}) {
		subtotal.TaxableAmount = ubl.Amount{Value: base.String(), CurrencyID: &currency}
	}
	// Computed early because F-LIB373 gates the dual-currency amount on the category.
	catID := taxCategoryID(r.Key)
	subtotal.TransactionCurrencyTaxAmount = transactionTax(accRate, catID, r.Amount, rCurrency)
	taxCat := ubl.TaxCategory{}

	if catID != "" {
		taxCat.ID = &ubl.IDType{Value: catID}
	}
	if v := r.Ext.Get(cef.ExtKeyVATEX).String(); v != "" {
		taxCat.TaxExemptionReasonCode = &v
	}
	if inv.Tax != nil {
		if note := findTaxNote(inv.Tax.Notes, cat.Code, r); note != nil {
			taxCat.TaxExemptionReason = &note.Text
		}
	}
	if r.Percent != nil {
		p := r.Percent.StringWithoutSymbol()
		taxCat.Percent = &p
	} else if taxCat.ID == nil || taxCat.ID.Value != "O" {
		p := "0"
		taxCat.Percent = &p
	}
	if cat.Code != cbc.CodeEmpty {
		taxCat.TaxScheme = &ubl.TaxScheme{ID: ubl.IDType{Value: cat.Code.String()}}
	}
	subtotal.TaxCategory = taxCat
	return subtotal
}

// findTaxNote copies gobl.ubl's own findTaxNote (totals.go); the only delta is
// matching by the tax.Note category + VAT key instead of the UNTDID category
// extension. Re-diff against gobl.ubl on every version bump.
func findTaxNote(notes []*tax.Note, catCode cbc.Code, rate *tax.RateTotal) *tax.Note {
	for _, n := range notes {
		if n.Category == catCode && n.Key == rate.Key {
			return n
		}
	}
	return nil
}

// transactionTax restates a StandardRated subtotal's tax in the tax currency (F-LIB373), or nil if there isn't one.
func transactionTax(accRate *cur.ExchangeRate, catID string, amount num.Amount, currencyID string) *ubl.Amount {
	if accRate == nil || catID != taxCategoryStandardRated {
		return nil
	}
	return &ubl.Amount{Value: accRate.Convert(amount).String(), CurrencyID: &currencyID}
}

func hasStandardRated(inv *bill.Invoice) bool {
	if inv.Totals == nil || inv.Totals.Taxes == nil {
		return false
	}
	for _, cat := range inv.Totals.Taxes.Categories {
		for _, r := range cat.Rates {
			if taxCategoryID(r.Key) == taxCategoryStandardRated && r.Percent != nil {
				return true
			}
		}
	}
	return false
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
func sumTaxTotalAmounts(totals []ubl.TaxTotal) ubl.Amount {
	if len(totals) == 0 {
		return ubl.Amount{}
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
	return ubl.Amount{Value: sum.String(), CurrencyID: totals[0].TaxAmount.CurrencyID}
}
