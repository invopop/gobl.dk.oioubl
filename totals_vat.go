package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/cef"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	cur "github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

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
		for _, r := range cat.Rates {
			if isExciseOnlyRate(cat, r, exciseBases) {
				continue
			}
			subtotal := buildVATSubtotal(inv, cat, r, accRate, rCurrency, currency)
			ui.TaxTotal[0].TaxSubtotal = append(ui.TaxTotal[0].TaxSubtotal, subtotal)
		}
	}
}

// isExciseOnlyRate reports whether a VAT rate row is owed entirely to excise
// and so has no subtotal of its own (F-LIB404); its type travels on the
// excise TaxTotal instead.
func isExciseOnlyRate(cat *tax.CategoryTotal, r *tax.RateTotal, exciseBases map[cbc.Key]num.Amount) bool {
	if cat.Code != tax.CategoryVAT || !r.Amount.IsZero() {
		return false
	}
	base, ok := exciseBases[r.Key]
	return ok && r.Base.Compare(base.Rescale(r.Base.Exp())) == 0
}

// buildVATSubtotal builds one cac:TaxSubtotal for a VAT rate row; percent is
// required unless the category is "O" (outside scope).
func buildVATSubtotal(inv *bill.Invoice, cat *tax.CategoryTotal, r *tax.RateTotal, accRate *cur.ExchangeRate, rCurrency, currency string) TaxSubtotal {
	subtotal := TaxSubtotal{
		TaxAmount: Amount{Value: r.Amount.String(), CurrencyID: &currency},
	}
	if r.Base != (num.Amount{}) {
		subtotal.TaxableAmount = Amount{Value: r.Base.String(), CurrencyID: &currency}
	}
	// Computed early because F-LIB373 gates the dual-currency amount on the category.
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
	if r.Percent != nil {
		p := r.Percent.StringWithoutSymbol()
		taxCat.Percent = &p
	} else if taxCat.ID == nil || taxCat.ID.Value != "O" {
		p := "0"
		taxCat.Percent = &p
	}
	if cat.Code != cbc.CodeEmpty {
		taxCat.TaxScheme = &TaxScheme{ID: IDType{Value: cat.Code.String()}}
	}
	subtotal.TaxCategory = taxCat
	return subtotal
}

// OIOUBL: maps the 63/Moms scheme and taxcategoryid-1.1 values back via goblTaxSchemeCategory/goblTaxCategoryCode.
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

// OIOUBL: matches by the tax.Note category + VAT key rather than the UNTDID category extension.
func findTaxNote(notes []*tax.Note, catCode cbc.Code, rate *tax.RateTotal) *tax.Note {
	for _, n := range notes {
		if n.Category == catCode && n.Key == rate.Key {
			return n
		}
	}
	return nil
}

// transactionTax restates a subtotal's tax in the tax currency, returning nil
// for single-currency invoices and non-StandardRated categories (F-LIB373).
func transactionTax(accRate *cur.ExchangeRate, catID string, amount num.Amount, currencyID string) *Amount {
	if accRate == nil || catID != taxCategoryStandardRated {
		return nil
	}
	return &Amount{Value: accRate.Convert(amount).String(), CurrencyID: &currencyID}
}

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
