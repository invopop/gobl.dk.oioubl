package oioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
)

// unclTaxCategoryCode maps an OIOUBL category to its UNTDID code. Anything
// unrecognised passes through untouched rather than failing.
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

// oioublVATKey maps an OIOUBL category to the GOBL VAT key, for a document-level
// duty's tax type, which has no route through the generic parse.
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

// stripTaxCategory rewrites a tax category back to its generic EN 16931 form.
func stripTaxCategory(tc *ubl.TaxCategory) {
	if tc == nil {
		return
	}
	if tc.ID != nil {
		tc.ID.Value = unclTaxCategoryCode(tc.ID.Value)
	}
	stripTaxScheme(tc.TaxScheme)
}

// stripClassifiedTaxCategory is stripTaxCategory's ClassifiedTaxCategory twin.
func stripClassifiedTaxCategory(tc *ubl.ClassifiedTaxCategory) {
	if tc == nil {
		return
	}
	if tc.ID != nil {
		tc.ID.Value = unclTaxCategoryCode(tc.ID.Value)
	}
	stripTaxScheme(tc.TaxScheme)
}

// stripTaxScheme rewrites OIOUBL's "63"/Moms VAT code back to plain "VAT".
func stripTaxScheme(ts *ubl.TaxScheme) {
	if ts == nil {
		return
	}
	if ts.ID.Value == taxSchemeVATCode {
		ts.ID.Value = string(tax.CategoryVAT)
	}
}

// stripTaxTotalCategories strips every subtotal of a tax total, which by this
// point holds only VAT.
func stripTaxTotalCategories(tt *ubl.TaxTotal) {
	for i := range tt.TaxSubtotal {
		stripTaxCategory(&tt.TaxSubtotal[i].TaxCategory)
	}
}
