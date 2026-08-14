package addon

import (
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// normalizeTaxCombo strips the EN 16931 UNTDID tax-category ext; the OIOUBL code
// derives from the GOBL VAT key (set by en16931, which runs first), so it's lossless.
func normalizeTaxCombo(c *tax.Combo) {
	c.Ext = c.Ext.Delete(untdid.ExtKeyTaxCategory)
}

// taxComboRules validates every VAT tax.Combo in the document.
func taxComboRules() *rules.Set {
	return rules.For(new(tax.Combo),
		// OIOUBL uses its own taxcategoryid, so the normalizer drops the UNTDID
		// tax-category ext; skip the EN 16931 rules that require and code-check it.
		rules.Ignore("GOBL-EU-EN16931-TAX-COMBO-01", "GOBL-EU-EN16931-TAX-COMBO-02"),
		rules.Field("key",
			rules.AssertIfPresent("01", "OIOUBL has no exempt VAT category (taxcategoryid-1.1)",
				is.NotIn(tax.KeyExempt)),
		),
	)
}
