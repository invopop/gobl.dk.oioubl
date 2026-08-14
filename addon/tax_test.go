package addon_test

import (
	"testing"

	oioubl "github.com/invopop/gobl.dk.oioubl/addon"
	en16931 "github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/catalogues/cef"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExemptVATIsRejected checks that an exempt VAT category is refused rather
// than quietly relabelled: OIOUBL's taxcategoryid-1.1 has no exempt value, so a
// seller must state the supply as zero-rated themselves.
func TestExemptVATIsRejected(t *testing.T) {
	inv := testInvoiceStandard(t)
	inv.Addons = tax.WithAddons(en16931.V2017, oioubl.V2)
	inv.Lines[0].Taxes = tax.Set{{
		Category: "VAT",
		Key:      tax.KeyExempt,
		Ext:      tax.ExtensionsOf(cbc.CodeMap{cef.ExtKeyVATEX: "VATEX-EU-132"}),
	}}
	inv.Payment = bankPayment()
	require.NoError(t, inv.Calculate())

	assert.ErrorContains(t, rules.Validate(inv), "OIOUBL has no exempt VAT category")
}

// TestNormalizeReverseChargeNeedsNoReason confirms that, with the OIOUBL addon present,
// EN 16931's exemption-reason requirement is relaxed for reverse-charge: OIOUBL
// reports it as the ReverseCharge category, which carries no exemption reason.
func TestNormalizeReverseChargeNeedsNoReason(t *testing.T) {
	inv := testInvoiceStandard(t)
	inv.Addons = tax.WithAddons(en16931.V2017, oioubl.V2)
	inv.Lines[0].Taxes = tax.Set{{Category: "VAT", Key: tax.KeyReverseCharge}}
	inv.Payment = bankPayment()
	require.NoError(t, inv.Calculate())
	assert.NoError(t, rules.Validate(inv))
	assert.Equal(t, tax.KeyReverseCharge, inv.Lines[0].Taxes[0].Key)
}

// TestNormalizeStandardUnchanged confirms the normalizer does not change the VAT key for standard VAT.
func TestNormalizeStandardUnchanged(t *testing.T) {
	inv := testInvoiceStandard(t)
	inv.Addons = tax.WithAddons(en16931.V2017, oioubl.V2)
	inv.Payment = bankPayment()
	require.NoError(t, inv.Calculate())
	assert.Equal(t, tax.KeyStandard, inv.Lines[0].Taxes[0].Key)
	require.NoError(t, rules.Validate(inv))
}
