package addon

import (
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/tax"
)

// normalizeTaxCombo strips the EN 16931 UNTDID tax-category ext; the OIOUBL code
// derives from the GOBL VAT key (set by en16931, which runs first), so it's lossless.
func normalizeTaxCombo(c *tax.Combo) {
	c.Ext = c.Ext.Delete(untdid.ExtKeyTaxCategory)
}

// normalizeTaxNote strips the same UNTDID tax-category extension from a tax note;
// the note's key identifies the rate it applies to.
func normalizeTaxNote(n *tax.Note) {
	n.Ext = n.Ext.Delete(untdid.ExtKeyTaxCategory)
}
