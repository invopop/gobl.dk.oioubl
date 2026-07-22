package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/cbc"
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

// taxCategoryID maps a GOBL VAT key to its OIOUBL taxcategoryid-1.1 code. OIOUBL
// 2.1 has no exempt category, so exempt reports as ZeroRated; keys with no
// OIOUBL category (export/intra-community/outside-scope) return "".
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
