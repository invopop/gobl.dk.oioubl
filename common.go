package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
)

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

// goblVATKey maps an OIOUBL taxcategoryid-1.1 value (also used by
// taxtypecode-1.1 for the named VAT types) back to a GOBL VAT key on parse.
// taxcategoryid-1.1 is the only version of that codelist — unrelated to the
// taxschemeid-1.x versioning of TaxScheme/ID. ZeroRated becomes "zero" (not
// "exempt"), matching the line-parse path, where a ZeroRated category's 0%
// percent is what GOBL normalizes into the zero key; unknown values return "".
func goblVATKey(id string) cbc.Key {
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
