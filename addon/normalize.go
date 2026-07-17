package addon

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
)

// normalizeInvoice defaults to GOBL's currency rounding rule to match OIOUBL's
// own rounding (F-INV128/F-INV133).
func normalizeInvoice(inv *bill.Invoice) {
	if inv.Tax == nil {
		inv.Tax = new(bill.Tax)
	}
	if inv.Tax.Rounding == "" {
		inv.Tax.Rounding = tax.RoundingRuleCurrency
	}
}

// normalizeTaxCombo strips the EN 16931 UNTDID tax-category ext (it's
// re-derivable from the GOBL key) and sets ExtKeyTaxCategory in its place.
func normalizeTaxCombo(c *tax.Combo) {
	c.Ext = c.Ext.Delete(untdid.ExtKeyTaxCategory)
	if cat := oioublTaxCategory(c.Key); cat != "" {
		c.Ext = c.Ext.Set(ExtKeyTaxCategory, cat)
	}
}

// oioublTaxCategory maps a GOBL VAT key to its OIOUBL taxcategoryid-1.1 code;
// exempt maps to ZeroRated since OIOUBL 2.1 has no exempt category (F-LIB309).
func oioublTaxCategory(key cbc.Key) cbc.Code {
	switch key {
	case tax.KeyStandard, "":
		return TaxCategoryStandardRated
	case tax.KeyZero, tax.KeyExempt:
		return TaxCategoryZeroRated
	case tax.KeyReverseCharge:
		return TaxCategoryReverseCharge
	}
	return ""
}

// normalizeTaxNote strips the same UNTDID tax-category extension from a tax note;
// the note's key identifies the rate it applies to.
func normalizeTaxNote(n *tax.Note) {
	n.Ext = n.Ext.Delete(untdid.ExtKeyTaxCategory)
}

// taxCategoryMapsToOIOUBL reports whether a GOBL VAT key has an OIOUBL
// taxcategoryid-1.1 equivalent (standard/zero/exempt/reverse-charge).
func taxCategoryMapsToOIOUBL(key cbc.Key) bool {
	return oioublTaxCategory(key) != ""
}
