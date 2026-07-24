package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
)

// Maps an OIOUBL taxcategoryid-1.1 value to its UNTDID 5305 code (S/Z/AE);
// an unrecognized value round-trips unchanged rather than failing.
func unclTaxCategoryCode(id string) string {
	switch id {
	case taxCategoryStandardRated:
		return "S"
	case taxCategoryZeroRated:
		return "Z"
	case taxCategoryReverseCharge:
		return "AE"
	}
	return id
}

// Only for a document-level excise duty's TaxTypeCode, which has no wire
// path through the generic parse.
func oioublVATKey(id string) cbc.Key {
	switch id {
	case taxCategoryStandardRated:
		return tax.KeyStandard
	case taxCategoryZeroRated:
		return tax.KeyZero
	case taxCategoryReverseCharge:
		return tax.KeyReverseCharge
	}
	return ""
}

// stripTaxCategoryWire rewrites a wire TaxCategory's symbolic OIOUBL ID and
// VAT TaxScheme back to their generic EN16931 form, in place.
func stripTaxCategoryWire(tc *ubl.TaxCategory) {
	if tc == nil {
		return
	}
	if tc.ID != nil {
		tc.ID.Value = unclTaxCategoryCode(tc.ID.Value)
	}
	stripTaxSchemeWire(tc.TaxScheme)
}

// stripClassifiedTaxCategoryWire is stripTaxCategoryWire's ClassifiedTaxCategory twin.
func stripClassifiedTaxCategoryWire(tc *ubl.ClassifiedTaxCategory) {
	if tc == nil {
		return
	}
	if tc.ID != nil {
		tc.ID.Value = unclTaxCategoryCode(tc.ID.Value)
	}
	stripTaxSchemeWire(tc.TaxScheme)
}

// Rewrites OIOUBL's "63"/Moms VAT tax-scheme code back to gobl.ubl's generic "VAT".
func stripTaxSchemeWire(ts *ubl.TaxScheme) {
	if ts == nil {
		return
	}
	if ts.ID.Value == taxSchemeVATCode {
		ts.ID.Value = string(tax.CategoryVAT)
	}
}

// stripTaxTotalCategories applies stripTaxCategoryWire to every subtotal in a
// (by this point, excise-free) cac:TaxTotal block.
func stripTaxTotalCategories(tt *ubl.TaxTotal) {
	for i := range tt.TaxSubtotal {
		stripTaxCategoryWire(&tt.TaxSubtotal[i].TaxCategory)
	}
}
