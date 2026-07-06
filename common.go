package dkoioubl

import (
	"errors"
	"strconv"
	"strings"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/validation"
)

// extFieldKey is the JSON path segment for the `ext` field on GOBL structs,
// used when constructing validation.Errors trees.
const extFieldKey = "ext"

// ptr returns a pointer to v. It keeps the many optional *string XML attributes
// (schemeID, listID, …) legible: ptr(schemeTaxScheme) rather than a throwaway
// local variable per attribute.
func ptr[T any](v T) *T { return &v }

func getTypeCode(inv *bill.Invoice) (string, error) {
	if inv.Tax == nil || inv.Tax.Ext.Get(untdid.ExtKeyDocumentType).String() == "" {
		return "", validation.Errors{
			"tax": validation.Errors{
				extFieldKey: validation.Errors{
					untdid.ExtKeyDocumentType.String(): errors.New("required"),
				},
			},
		}
	}
	return inv.Tax.Ext.Get(untdid.ExtKeyDocumentType).String(), nil
}

// buildTaxCategoryKey keys a tax category by scheme+category, plus the normalized
// percent for "S" (one invoice can carry several standard-rate subtotals).
func buildTaxCategoryKey(taxSchemeID, categoryID string, percent *string) string {
	if categoryID == "S" {
		return taxSchemeID + ":" + categoryID + ":" + normalizeTaxPercent(percent)
	}
	return taxSchemeID + ":" + categoryID
}

// normalizeTaxPercent converts a percent string to a canonical form by stripping trailing zeros,
// so that "20", "20.0", and "20.00" all map to "20".
func normalizeTaxPercent(percent *string) string {
	if percent == nil {
		return ""
	}
	s := normalizeNumericString(*percent)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// normalizeNumericString cleans up numeric strings to ensure they can be parsed correctly.
// It handles:
// - Leading/trailing whitespace (e.g., " 123.45 " -> "123.45")
// - Numbers starting with decimal point (e.g., ".07" -> "0.07")
func normalizeNumericString(s string) string {
	// Trim whitespace
	s = strings.TrimSpace(s)

	// Add leading zero if string starts with decimal point
	if strings.HasPrefix(s, ".") {
		s = "0" + s
	}

	return s
}

// goblTaxSchemeCategory maps the OIOUBL VAT scheme "63" (Moms) back to GOBL's
// "VAT" on parse; any other value passes through unchanged.
func goblTaxSchemeCategory(schemeID string) cbc.Code {
	if schemeID == taxSchemeVATCode {
		return cbc.Code(ubl.TaxSchemeVAT)
	}
	return cbc.Code(schemeID)
}

// goblTaxCategoryCode maps an OIOUBL taxcategoryid-1.1 value to the UNTDID 5305
// code (S/Z/AE) that en16931 reads on parse to derive the GOBL tax key.
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
