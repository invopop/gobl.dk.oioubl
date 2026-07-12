package dkoioubl

import (
	"strconv"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/cef"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	cur "github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

// addMonetaryTotal rebuilds LegalMonetaryTotal for OIOUBL's gross line amounts
// (F-INV348), folding line allowances/charges into the document totals.
func (ui *Invoice) addMonetaryTotal(inv *bill.Invoice, currency string) {
	t := inv.Totals
	exp := t.Sum.Exp()
	grossSum := num.MakeAmount(0, exp)
	lineDiscounts := num.MakeAmount(0, exp)
	lineCharges := num.MakeAmount(0, exp)
	// excise holds duty charges OIOUBL emits as cac:TaxTotal/Excise subtotals; they
	// leave the AllowanceCharge/ChargeTotalAmount path and land in
	// TaxInclusiveAmount as tax instead (the subtotals are built in addTotals).
	excise := num.MakeAmount(0, exp)
	for _, l := range inv.Lines {
		if l.Sum != nil {
			grossSum = grossSum.Add(rescaleToCurrency(*l.Sum, currency))
		}
		for _, d := range l.Discounts {
			lineDiscounts = lineDiscounts.Add(d.Amount)
		}
		ordinary := make([]*bill.LineCharge, 0, len(l.Charges))
		for _, c := range l.Charges {
			if chargeExciseScheme(c.Key) != "" {
				excise = excise.Add(rescaleToCurrency(c.Amount, currency))
				continue
			}
			lineCharges = lineCharges.Add(c.Amount)
			ordinary = append(ordinary, c)
		}
		// Promote ordinary line allowances/charges to document-level AllowanceCharge
		// so they sum to Allowance/ChargeTotalAmount (F-INV129/F-INV130).
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
		if chargeExciseScheme(ch.Key) != "" {
			a := rescaleToCurrency(ch.Amount, currency)
			excise = excise.Add(a)
			chg = chg.Subtract(a) // counted in t.Charge above; OIOUBL emits it as tax
		}
	}
	if !chg.IsZero() {
		ui.LegalMonetaryTotal.ChargeTotalAmount = &Amount{Value: chg.String(), CurrencyID: &currency}
	}
	// OIOUBL rounds per line then sums (F-INV128/F-INV133); GOBL end-rounds,
	// which can differ by a cent on fractional quantities. Recompute the
	// inclusive/payable totals from the rounded components so they reconcile.
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

// addPrepaidPayments emits a cac:PrepaidPayment per GOBL advance. OIOUBL
// requires the PaidAmount elements to sum to LegalMonetaryTotal/PrepaidAmount
// (F-INV131).
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
	rCurrency := inv.RegimeDef().Currency.String()

	ui.LegalMonetaryTotal = MonetaryTotal{
		LineExtensionAmount: Amount{Value: t.Sum.String(), CurrencyID: &currency},
		TaxExclusiveAmount:  Amount{Value: t.Total.String(), CurrencyID: &currency},
		TaxInclusiveAmount:  Amount{Value: t.TotalWithTax.String(), CurrencyID: &currency},
		PayableAmount:       &Amount{Value: t.Payable.String(), CurrencyID: &currency},
	}

	if t.Discount != nil {
		ui.LegalMonetaryTotal.AllowanceTotalAmount = &Amount{Value: t.Discount.String(), CurrencyID: &currency}
	}
	if t.Charge != nil {
		ui.LegalMonetaryTotal.ChargeTotalAmount = &Amount{Value: t.Charge.String(), CurrencyID: &currency}
	}

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

	var accRate *cur.ExchangeRate
	if inv.Currency != inv.RegimeDef().Currency {
		accRate = cur.MatchExchangeRate(inv.ExchangeRates, inv.Currency, inv.RegimeDef().Currency)
	}

	if t.Taxes != nil && len(t.Taxes.Categories) > 0 {
		for _, cat := range t.Taxes.Categories {
			for _, r := range cat.Rates {
				subtotal := TaxSubtotal{
					TaxAmount: Amount{Value: r.Amount.String(), CurrencyID: &currency},
				}
				if r.Base != (num.Amount{}) {
					subtotal.TaxableAmount = Amount{Value: r.Base.String(), CurrencyID: &currency}
				}
				// The category is mapped from the GOBL key. Computed early because
				// F-LIB373 gates the dual-currency amount on it.
				catID := taxCategoryID(r.Key)
				subtotal.TransactionCurrencyTaxAmount = transactionTax(accRate, catID, r.Amount, rCurrency)
				taxCat := TaxCategory{}

				if catID != "" {
					taxCat.ID = &IDType{Value: catID}
				}
				if v := r.Ext.Get(cef.ExtKeyVATEX).String(); v != "" {
					taxCat.TaxExemptionReasonCode = &v
				}

				if inv.Tax != nil {
					if note := findTaxNote(inv.Tax.Notes, cat.Code, r); note != nil {
						taxCat.TaxExemptionReason = &note.Text
					}
				}

				// Set percent: required unless category is "O" (outside scope)
				if r.Percent != nil {
					p := r.Percent.StringWithoutSymbol()
					taxCat.Percent = &p
				} else if taxCat.ID == nil || taxCat.ID.Value != "O" {
					// Default to 0% when not outside scope
					p := "0"
					taxCat.Percent = &p
				}

				if cat.Code != cbc.CodeEmpty {
					taxCat.TaxScheme = &TaxScheme{ID: IDType{Value: cat.Code.String()}}
				}
				subtotal.TaxCategory = taxCat
				ui.TaxTotal[0].TaxSubtotal = append(ui.TaxTotal[0].TaxSubtotal, subtotal)
			}
		}
	}

	// Non-VAT excise duties travel as their own cac:TaxTotal blocks (the VAT total
	// already includes them in its base, GOBL having folded the charge into the
	// line). applyTotals sums every TaxTotal into TaxExclusiveAmount.
	ui.TaxTotal = append(ui.TaxTotal, makeExciseTaxTotals(collectExcise(inv, currency), currency)...)
}

// taxCategoryInfo holds tax category information from TaxTotal
type taxCategoryInfo struct {
	exemptionReasonCode string
}

// buildTaxCategoryMap builds a map of tax category information from TaxTotal.
func (ui *Invoice) buildTaxCategoryMap() map[string]*taxCategoryInfo {
	categoryMap := make(map[string]*taxCategoryInfo)

	for _, taxTotal := range ui.TaxTotal {
		for _, subtotal := range taxTotal.TaxSubtotal {
			if subtotal.TaxCategory.ID != nil && subtotal.TaxCategory.TaxScheme != nil {
				key := ubl.BuildTaxCategoryKey(subtotal.TaxCategory.TaxScheme.ID.Value, subtotal.TaxCategory.ID.Value, subtotal.TaxCategory.Percent)
				info := &taxCategoryInfo{}
				if subtotal.TaxCategory.TaxExemptionReasonCode != nil {
					info.exemptionReasonCode = *subtotal.TaxCategory.TaxExemptionReasonCode
				}
				categoryMap[key] = info
			}
		}
	}

	return categoryMap
}

// goblAddTaxNotes extracts tax notes from UBL TaxTotal subtotals and adds them
// to the invoice's Tax.Notes.
func (ui *Invoice) goblAddTaxNotes(inv *bill.Invoice) {
	for _, tt := range ui.TaxTotal {
		for _, st := range tt.TaxSubtotal {
			tc := st.TaxCategory
			if tc.TaxExemptionReason == nil || tc.ID == nil || tc.TaxScheme == nil {
				continue
			}
			note := &tax.Note{
				Category: goblTaxSchemeCategory(tc.TaxScheme.ID.Value),
				Text:     ubl.CleanString(*tc.TaxExemptionReason),
				Ext:      tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyTaxCategory: goblTaxCategoryCode(tc.ID.Value)}),
			}
			inv.Tax = inv.Tax.MergeNotes(note)
		}
	}
}

// findTaxNote finds a tax note that matches the given category code and rate
// total by category and VAT key — the same pair tax.Note uses to identify
// itself. Matching on the key works across profiles and survives the dk-oioubl
// addon stripping the UNTDID extension from the document.
func findTaxNote(notes []*tax.Note, catCode cbc.Code, rate *tax.RateTotal) *tax.Note {
	for _, n := range notes {
		if n.Category == catCode && n.Key == rate.Key {
			return n
		}
	}
	return nil
}

// goblExchangeRates derives the exchange rate between DocumentCurrencyCode and
// TaxCurrencyCode. OIOUBL carries the accounting-currency tax per subtotal as
// TransactionCurrencyTaxAmount; a second TaxTotal block (the EN16931/Peppol
// shape) is also read for inbound documents that use it.
func goblExchangeRates(docCurrency, taxCurrency cur.Code, taxTotals []TaxTotal) []*cur.ExchangeRate {
	if len(taxTotals) == 0 {
		return nil
	}

	docAmount, err := num.AmountFromString(ubl.NormalizeNumericString(taxTotals[0].TaxAmount.Value))
	if err != nil || docAmount.IsZero() {
		return nil
	}

	taxAmount, ok := taxCurrencyTaxAmount(taxTotals)
	if !ok {
		return nil
	}

	rate := taxAmount.Divide(docAmount)

	return []*cur.ExchangeRate{
		{
			From:   docCurrency,
			To:     taxCurrency,
			Amount: rate,
		},
	}
}

// taxCurrencyTaxAmount returns the total tax expressed in the tax currency,
// summing the per-subtotal TransactionCurrencyTaxAmount of the first TaxTotal,
// or reading a second TaxTotal block when present.
func taxCurrencyTaxAmount(taxTotals []TaxTotal) (num.Amount, bool) {
	if len(taxTotals) >= 2 {
		a, err := num.AmountFromString(ubl.NormalizeNumericString(taxTotals[1].TaxAmount.Value))
		if err != nil {
			return num.Amount{}, false
		}
		return a, true
	}

	var total num.Amount
	found := false
	for _, st := range taxTotals[0].TaxSubtotal {
		if st.TransactionCurrencyTaxAmount == nil {
			continue
		}
		a, err := num.AmountFromString(ubl.NormalizeNumericString(st.TransactionCurrencyTaxAmount.Value))
		if err != nil {
			return num.Amount{}, false
		}
		if found {
			total = total.Add(a)
		} else {
			total, found = a, true
		}
	}
	return total, found
}

// OIOUBL taxcategoryid-1.1 category codes and the serialization-only
// taxschemeid-1.1 VAT (Moms) code.
const (
	taxCategoryStandardRated = "StandardRated"
	taxCategoryZeroRated     = "ZeroRated"
	taxCategoryReverseCharge = "ReverseCharge"

	taxSchemeVATCode = "63" // taxschemeid-1.1 VAT (Moms)
)

// transactionTax restates a subtotal's tax in the tax currency. F-LIB373
// allows it only on StandardRated, so it returns nil otherwise (and for
// single-currency invoices).
func transactionTax(accRate *cur.ExchangeRate, catID string, amount num.Amount, currencyID string) *Amount {
	if accRate == nil || catID != taxCategoryStandardRated {
		return nil
	}
	return &Amount{Value: accRate.Convert(amount).String(), CurrencyID: &currencyID}
}

// hasStandardRated reports whether the invoice carries a StandardRated VAT
// combo. cbc:TransactionCurrencyTaxAmount is emitted only on StandardRated
// subtotals (F-LIB373), so cbc:TaxCurrencyCode — which then requires at least one
// of those amounts (F-INV018) — must be suppressed when none is present.
func hasStandardRated(inv *bill.Invoice) bool {
	if inv.Totals == nil || inv.Totals.Taxes == nil {
		return false
	}
	for _, cat := range inv.Totals.Taxes.Categories {
		for _, r := range cat.Rates {
			if taxCategoryID(r.Key) == taxCategoryStandardRated {
				return true
			}
		}
	}
	return false
}

// taxCategoryID maps a GOBL VAT key to the OIOUBL taxcategoryid-1.1 code
// emitted as cac:TaxCategory/cbc:ID. OIOUBL 2.1 has no exempt category, so exempt
// reports as ZeroRated. Returns "" for keys with no OIOUBL category (export,
// intra-community and outside-scope, which the addon rejects upstream).
func taxCategoryID(key cbc.Key) string {
	switch key {
	case tax.KeyStandard, "":
		return taxCategoryStandardRated
	case tax.KeyZero, tax.KeyExempt:
		return taxCategoryZeroRated
	case tax.KeyReverseCharge:
		return taxCategoryReverseCharge
	}
	return ""
}

// applyTotals stamps the taxcategoryid attributes on the document-level tax
// subtotals and allowance/charges, and re-interprets TaxExclusiveAmount as the
// total tax amount (F-INV127), not the pre-tax sum as in generic UBL. It runs
// after the whole document is assembled because the document allowance set is
// only complete once promoted line allowances have been added.
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

// sumTaxTotalAmounts totals the TaxAmount of every cac:TaxTotal. With a single
// (VAT) total it returns that amount unchanged; excise totals add to it.
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

// stampTaxCategoryID stamps the taxcategoryid-1.1 codelist attributes onto a
// tax-category cbc:ID, defaulting an absent category to StandardRated.
func stampTaxCategoryID(id *IDType) *IDType {
	if id == nil {
		id = &IDType{Value: taxCategoryStandardRated}
	}
	id.SchemeID = ptr(schemeTaxCategory)
	id.SchemeAgencyID = ptr(agencyID)
	return id
}

func applyTaxCategory(tc *TaxCategory) {
	if tc == nil {
		return
	}
	tc.ID = stampTaxCategoryID(tc.ID)
	applyTaxScheme(tc.TaxScheme)
}

func applyClassifiedTaxCategory(tc *ClassifiedTaxCategory) {
	if tc == nil {
		return
	}
	tc.ID = stampTaxCategoryID(tc.ID)
	applyTaxScheme(tc.TaxScheme)
}

func applyTaxScheme(ts *TaxScheme) {
	if ts == nil {
		return
	}
	ts.ID = IDType{
		SchemeID:       ptr(schemeTaxScheme),
		SchemeAgencyID: ptr(agencyID),
		Value:          taxSchemeVATCode,
	}
	ts.Name = ptr("Moms")
}
