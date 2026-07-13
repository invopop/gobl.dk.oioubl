package dkoioubl

import (
	"errors"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/validation"
)

// extFieldKey is the JSON path segment for the `ext` field, used in validation.Errors trees.
const extFieldKey = "ext"

// ptr returns a pointer to v, keeping the optional *string XML attributes legible.
func ptr[T any](v T) *T { return &v }

// Identical to gobl.ubl.getTypeCode.
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
