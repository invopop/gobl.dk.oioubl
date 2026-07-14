package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/cbc"
)

// extFieldKey is the JSON path segment for the `ext` field, used in validation.Errors trees.
const extFieldKey = "ext"

// goblTaxSchemeCategory maps the OIOUBL VAT scheme "63" (Moms) back to GOBL's
// "VAT" on parse; any other value passes through unchanged.
func goblTaxSchemeCategory(schemeID string) cbc.Code {
	if schemeID == taxSchemeVATCode {
		return cbc.Code(ubl.TaxSchemeVAT)
	}
	return cbc.Code(schemeID)
}

// goblTaxCategoryCode maps an OIOUBL taxcategoryid-1.1 value to its UNTDID 5305 code (S/Z/AE).
func goblTaxCategoryCode(id string) cbc.Code {
	switch id {
	case taxCategoryStandardRated:
		return "S"
	case taxCategoryZeroRated:
		return "Z"
	case taxCategoryReverseCharge:
		return "AE"
	}
	return cbc.Code(id)
}
