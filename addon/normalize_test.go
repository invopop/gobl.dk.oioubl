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

// TestNormalizeExemptToZeroRated checks that a VAT-exempt line keeps its GOBL
// exempt category while carrying the OIOUBL ZeroRated category in the
// dk-oioubl-tax-category extension. A VATEX reason remains allowed (and is
// carried through), even though OIOUBL no longer requires one.
func TestNormalizeExemptToZeroRated(t *testing.T) {
	inv := testInvoiceStandard(t)
	inv.Addons = tax.WithAddons(en16931.V2017, oioubl.V2)
	inv.Lines[0].Taxes = tax.Set{{
		Category: "VAT",
		Key:      tax.KeyExempt,
		Ext:      tax.ExtensionsOf(cbc.CodeMap{cef.ExtKeyVATEX: "VATEX-EU-132"}),
	}}
	inv.Payment = bankPayment()
	require.NoError(t, inv.Calculate())

	assert.Equal(t, tax.KeyExempt, inv.Lines[0].Taxes[0].Key,
		"the GOBL category stays exempt; the converter maps it to OIOUBL ZeroRated")
	require.NoError(t, rules.Validate(inv))
}

// TestNormalizeExemptNeedsNoReason confirms that, with the OIOUBL addon present,
// EN 16931's exemption-reason requirement is relaxed: OIOUBL 2.1 has no exempt
// category (exempt is reported as ZeroRated, which requires no reason), so a
// VAT-exempt line with neither a VATEX code nor an exemption note validates.

// TestNormalizeReverseChargeNeedsNoReason confirms the same relaxation for
// reverse-charge: OIOUBL reports it as the ReverseCharge category, which carries
// no exemption reason, so the EN 16931 exemption-note requirement is skipped.
func TestNormalizeReverseChargeNeedsNoReason(t *testing.T) {
	inv := testInvoiceStandard(t)
	inv.Addons = tax.WithAddons(en16931.V2017, oioubl.V2)
	inv.Lines[0].Taxes = tax.Set{{Category: "VAT", Key: tax.KeyReverseCharge}}
	inv.Payment = bankPayment()
	require.NoError(t, inv.Calculate())
	assert.NoError(t, rules.Validate(inv))
	assert.Equal(t, tax.KeyReverseCharge, inv.Lines[0].Taxes[0].Key)
}

// TestNormalizeStandardUnchanged confirms the normalizer only touches exempt.
func TestNormalizeStandardUnchanged(t *testing.T) {
	inv := testInvoiceStandard(t)
	inv.Addons = tax.WithAddons(en16931.V2017, oioubl.V2)
	inv.Payment = bankPayment()
	require.NoError(t, inv.Calculate())
	assert.Equal(t, tax.KeyStandard, inv.Lines[0].Taxes[0].Key)
	require.NoError(t, rules.Validate(inv))
}

