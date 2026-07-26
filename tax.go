package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	cur "github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

// OIOUBL taxcategoryid-1.1 category codes; "Excise" (see excise.go) is the
// codelist's only other emitted value.
const (
	taxCategoryStandardRated = "StandardRated"
	taxCategoryZeroRated     = "ZeroRated"
	taxCategoryReverseCharge = "ReverseCharge"

	taxSchemeVATCode = "63" // taxschemeid-1.5 VAT (Moms)
)

// Turning GOBL tax keys into OIOUBL category and scheme identifiers.

// taxCategoryID maps a GOBL VAT key to its OIOUBL taxcategoryid-1.1 code (exempt maps to ZeroRated, since OIOUBL 2.1 has no exempt category; unsupported keys return "").
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

// stampTaxCategoryID stamps the taxcategoryid-1.1 attributes, defaulting an absent category to StandardRated.
func stampTaxCategoryID(id *ubl.IDType) *ubl.IDType {
	if id == nil {
		id = &ubl.IDType{Value: taxCategoryStandardRated}
	}
	schemeID := schemeTaxCategory
	schemeAgencyID := agencyID
	id.SchemeID = &schemeID
	id.SchemeAgencyID = &schemeAgencyID
	return id
}

func applyTaxCategory(tc *ubl.TaxCategory) {
	if tc == nil {
		return
	}
	tc.ID = stampTaxCategoryID(tc.ID)
	applyTaxScheme(tc.TaxScheme)
}

func applyClassifiedTaxCategory(tc *ubl.ClassifiedTaxCategory) {
	if tc == nil {
		return
	}
	tc.ID = stampTaxCategoryID(tc.ID)
	applyTaxScheme(tc.TaxScheme)
}

func applyTaxScheme(ts *ubl.TaxScheme) {
	if ts == nil {
		return
	}
	schemeID := schemeTaxScheme
	schemeAgencyID := agencyID
	ts.ID = ubl.IDType{
		SchemeID:       &schemeID,
		SchemeAgencyID: &schemeAgencyID,
		Value:          taxSchemeVATCode,
	}
	name := "Moms"
	ts.Name = &name
}

// Reading the tax data back off the GOBL invoice.

// findTaxNote ports gobl.ubl's own version, matching by GOBL VAT key instead of the UNTDID ext -- re-diff on version bumps.
func findTaxNote(notes []*tax.Note, catCode cbc.Code, rate *tax.RateTotal) *tax.Note {
	for _, n := range notes {
		if n.Category == catCode && n.Key == rate.Key {
			return n
		}
	}
	return nil
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

// The second tax currency, which OIOUBL only wants in one case.

// fixTaxCurrency drops the second currency unless a rate that charges tax uses
// it, which is the only case OIOUBL states it for (F-LIB373/F-INV018).
func (ui *Invoice) fixTaxCurrency(inv *bill.Invoice) {
	if ui.TaxCurrencyCode != "" && !hasStandardRated(inv) {
		ui.TaxCurrencyCode = ""
	}
}

// transactionTax restates a StandardRated subtotal's tax in the tax currency (F-LIB373), or nil if there isn't one.
func transactionTax(accRate *cur.ExchangeRate, catID string, amount num.Amount, currencyID string) *ubl.Amount {
	if accRate == nil || catID != taxCategoryStandardRated {
		return nil
	}
	return &ubl.Amount{Value: accRate.Convert(amount).String(), CurrencyID: &currencyID}
}
