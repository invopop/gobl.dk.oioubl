package oioubl

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

	// taxSchemeVATName and taxSchemeDutyName are the free-text names OIOUBL
	// puts on a tax scheme. Only the VAT pairing is fixed: "Moms" belongs to
	// code 63 and to nothing else (F-LIB198/199).
	taxSchemeVATName  = "Moms"
	taxSchemeDutyName = "Excise"
)

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
// stampTaxCategoryID stamps the taxcategoryid-1.1 list attributes. A nil ID means
// the GOBL VAT key has no OIOUBL equivalent (intra-community, export,
// outside-scope): leave it unset rather than guess, so F-LIB074 rejects the
// document instead of it going out mislabelled as StandardRated.
func stampTaxCategoryID(id *ubl.IDType) *ubl.IDType {
	if id == nil {
		return nil
	}
	id.SchemeID = ptr(schemeTaxCategory)
	id.SchemeAgencyID = ptr(agencyID)
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
	ts.ID = ubl.IDType{
		SchemeID:       ptr(schemeTaxScheme),
		SchemeAgencyID: ptr(agencyID),
		Value:          taxSchemeVATCode,
	}
	ts.Name = ptr(taxSchemeVATName)
}

// applyPartyTaxScheme stamps a party's tax scheme: VAT becomes "63"/Moms, and a
// duty keeps its own code, named after the matching charge where there is one.
func applyPartyTaxScheme(ts *ubl.TaxScheme, duties map[string]exciseDuty) {
	if ts == nil {
		return
	}
	code := ts.ID.Value
	if code == "" || code == taxSchemeVATCode || code == string(tax.CategoryVAT) {
		applyTaxScheme(ts)
		return
	}
	name, typeCode := taxSchemeDutyName, taxCategoryStandardRated
	if d, ok := duties[code]; ok {
		if d.name != "" {
			name = d.name
		}
		if d.typeCode != "" {
			typeCode = d.typeCode
		}
	}
	ts.ID = ubl.IDType{
		SchemeID:       ptr(schemeTaxScheme),
		SchemeAgencyID: ptr(agencyID),
		Value:          code,
	}
	// Any name but "Moms" is allowed (F-LIB198/199), but a scheme other than
	// VAT has to say which category it falls under (F-LIB197).
	ts.Name = ptr(name)
	ts.TaxTypeCode = &ubl.IDType{
		ListAgencyID: ptr(agencyID),
		ListID:       ptr(codelistTaxType),
		Value:        typeCode,
	}
}

// dutiesByCode indexes the duties the document charges, so a party registered
// for one can be described with the same name and category the charge uses.
func dutiesByCode(inv *bill.Invoice) map[string]exciseDuty {
	out := make(map[string]exciseDuty)
	for _, d := range collectExcise(inv, inv.Currency.String()) {
		if d.scheme == "" {
			continue
		}
		if _, seen := out[d.scheme]; !seen {
			out[d.scheme] = d
		}
	}
	return out
}

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
