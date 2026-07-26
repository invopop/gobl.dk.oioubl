package oioubl

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

// lineSums are the per-line amounts the document totals are built from.
type lineSums struct {
	gross     num.Amount
	discounts num.Amount
	charges   num.Amount
	excise    num.Amount // OIOUBL reports excise as tax, never as a charge
}

// categoryGroups says how a category's rates collapse into the one subtotal
// OIOUBL allows it (G27 3.5).
type categoryGroups struct {
	charging  map[string]bool       // categories with a rate that charges tax
	extraBase map[string]num.Amount // untaxed amounts riding along with it
}

func (ui *Invoice) buildTotals(inv *bill.Invoice) {
	if inv == nil || inv.Totals == nil {
		return
	}
	t := inv.Totals
	currency := inv.Currency.String()

	ui.createMonetaryTotal(inv, currency)
	ui.includePrepaidPayments(inv, currency)

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
	ui.appendVATSubtotals(inv, currency)

	// Non-VAT excise duties travel as their own cac:TaxTotal blocks (the VAT total
	// already includes them in its base); applyTotals sums them into TaxExclusiveAmount.
	ui.TaxTotal = append(ui.TaxTotal, makeExciseTaxTotals(collectExcise(inv, currency), currency)...)
}

// createMonetaryTotal rebuilds LegalMonetaryTotal with gross line amounts (F-INV348).
func (ui *Invoice) createMonetaryTotal(inv *bill.Invoice, currency string) {
	t := inv.Totals
	sums := ui.sumLines(inv, currency)
	amount := func(a num.Amount) *ubl.Amount {
		return &ubl.Amount{Value: a.String(), CurrencyID: &currency}
	}

	ui.LegalMonetaryTotal.LineExtensionAmount = *amount(sums.gross)

	allowances := sums.discounts
	if t.Discount != nil {
		allowances = allowances.Add(*t.Discount)
	}
	if !allowances.IsZero() {
		ui.LegalMonetaryTotal.AllowanceTotalAmount = amount(allowances)
	}

	// Document-level excise is already in t.Charge, but OIOUBL wants it as tax.
	charges, excise := sums.charges, sums.excise
	if t.Charge != nil {
		charges = charges.Add(*t.Charge)
	}
	for _, ch := range inv.Charges {
		if chargeIsExcise(ch.Key) {
			excise = excise.Add(ch.Amount)
			charges = charges.Subtract(ch.Amount)
		}
	}
	// A charge total left over purely from excise isn't real (F-INV128/130/133).
	if charges.IsZero() {
		ui.LegalMonetaryTotal.ChargeTotalAmount = nil
	} else {
		ui.LegalMonetaryTotal.ChargeTotalAmount = amount(charges)
	}

	// Recomputed because OIOUBL counts excise as tax, not as a charge
	// (F-INV128/F-INV133); GOBL's own totals have no field for that.
	inclusive := sums.gross.Add(t.Tax).Add(excise).Add(charges).Subtract(allowances)
	if t.Rounding != nil {
		inclusive = inclusive.Add(*t.Rounding)
	}
	ui.LegalMonetaryTotal.TaxInclusiveAmount = *amount(inclusive)

	payable := inclusive
	if t.Advances != nil {
		payable = payable.Subtract(*t.Advances)
	}
	ui.LegalMonetaryTotal.PayableAmount = amount(payable)
}

// sumLines totals the lines, and promotes their ordinary allowances and
// charges to document level (F-INV129/F-INV130).
func (ui *Invoice) sumLines(inv *bill.Invoice, currency string) lineSums {
	exp := inv.Totals.Sum.Exp()
	sums := lineSums{
		gross:     num.MakeAmount(0, exp),
		discounts: num.MakeAmount(0, exp),
		charges:   num.MakeAmount(0, exp),
		excise:    num.MakeAmount(0, exp),
	}
	for _, l := range inv.Lines {
		if l.Sum != nil {
			sums.gross = sums.gross.Add(*l.Sum)
		}
		for _, d := range l.Discounts {
			sums.discounts = sums.discounts.Add(d.Amount)
		}
		ordinary := make([]*bill.LineCharge, 0, len(l.Charges))
		for _, c := range l.Charges {
			if chargeIsExcise(c.Key) {
				sums.excise = sums.excise.Add(rescaleToCurrency(c.Amount, currency))
				continue
			}
			sums.charges = sums.charges.Add(c.Amount)
			ordinary = append(ordinary, c)
		}
		for _, ac := range makeLineCharges(ordinary, l.Discounts, currency, l.Sum, l.Taxes) {
			ui.AllowanceCharge = append(ui.AllowanceCharge, *ac)
		}
	}
	return sums
}

// includePrepaidPayments emits a cac:PrepaidPayment per GOBL advance (F-INV131).
func (ui *Invoice) includePrepaidPayments(inv *bill.Invoice, currency string) {
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
			d := formatDate(*adv.Date)
			pp.ReceivedDate = &d
		}
		if adv.Ref != "" {
			ref := adv.Ref
			pp.InstructionID = &ref
		}
		ui.PrepaidPayment = append(ui.PrepaidPayment, pp)
	}
}

// appendVATSubtotals builds one cac:TaxSubtotal per VAT rate row onto ui.TaxTotal[0].
func (ui *Invoice) appendVATSubtotals(inv *bill.Invoice, currency string) {
	t := inv.Totals
	if t.Taxes == nil || len(t.Taxes.Categories) == 0 {
		return
	}
	// GetCurrency is nil-safe: an invoice whose supplier country has no GOBL
	// regime still has to convert rather than panic.
	rCurrency := inv.RegimeDef().GetCurrency().String()
	var accRate *cur.ExchangeRate
	if inv.Currency != inv.RegimeDef().GetCurrency() {
		accRate = cur.MatchExchangeRate(inv.ExchangeRates, inv.Currency, inv.RegimeDef().GetCurrency())
	}
	exciseBases := exciseVATBases(inv)
	for _, cat := range t.Taxes.Categories {
		groups := groupRatesByCategory(cat.Rates)
		for _, r := range cat.Rates {
			catID := taxCategoryID(r.Key)
			if r.Percent == nil && r.Amount.IsZero() && groups.charging[catID] {
				// Folds into the category's real rate below (G17 3.1), unless it has none of its own (e.g. ReverseCharge).
				continue
			}
			if isExciseOnlyRate(cat, r, exciseBases) {
				continue
			}
			base := r.Base
			if extra, ok := groups.extraBase[catID]; ok {
				base = base.Add(extra.Rescale(base.Exp()))
			}
			subtotal := buildVATSubtotal(inv, cat, r, base, accRate, rCurrency, currency)
			ui.TaxTotal[0].TaxSubtotal = append(ui.TaxTotal[0].TaxSubtotal, subtotal)
		}
	}
}

// groupRatesByCategory works out that collapse: a rate charging no tax gets no
// subtotal of its own, so its amount joins the category's charging rate.
func groupRatesByCategory(rates []*tax.RateTotal) categoryGroups {
	g := categoryGroups{
		charging:  make(map[string]bool),
		extraBase: make(map[string]num.Amount),
	}
	for _, r := range rates {
		if r.Percent == nil && r.Amount.IsZero() {
			continue
		}
		if catID := taxCategoryID(r.Key); catID != "" {
			g.charging[catID] = true
		}
	}
	for _, r := range rates {
		if r.Percent != nil || !r.Amount.IsZero() {
			continue
		}
		catID := taxCategoryID(r.Key)
		if catID == "" || !g.charging[catID] {
			continue
		}
		if sum, ok := g.extraBase[catID]; ok {
			g.extraBase[catID] = sum.Add(r.Base.Rescale(sum.Exp()))
		} else {
			g.extraBase[catID] = r.Base
		}
	}
	return g
}

// buildVATSubtotal ports gobl.ubl's own subtotal builder
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

// applyTotals stamps taxcategoryid attributes and re-interprets TaxExclusiveAmount as the total tax (F-INV127).
func (ui *Invoice) applyTotals() {
	for i := range ui.TaxTotal {
		for j := range ui.TaxTotal[i].TaxSubtotal {
			st := &ui.TaxTotal[i].TaxSubtotal[j]
			// Excise subtotals carry their own scheme code, name and TaxTypeCode
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
	sum, err := num.AmountFromString(normalizeNumericString(totals[0].TaxAmount.Value))
	if err != nil {
		return totals[0].TaxAmount
	}
	for _, tt := range totals[1:] {
		a, err := num.AmountFromString(normalizeNumericString(tt.TaxAmount.Value))
		if err != nil {
			continue
		}
		sum = sum.Add(a.Rescale(sum.Exp()))
	}
	return ubl.Amount{Value: sum.String(), CurrencyID: totals[0].TaxAmount.CurrencyID}
}
