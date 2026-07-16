package addon_test

import (
	"testing"

	oioubl "github.com/invopop/gobl.dk.oioubl/addon"
	en16931 "github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/cef"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bankPayment() *bill.PaymentDetails {
	return &bill.PaymentDetails{
		Terms: &pay.Terms{Notes: "Net 30 days"},
		Instructions: &pay.Instructions{
			// MeansKeyDebitTransfer maps to UNTDID code 31, the only OIOUBL-valid
			// bank-transfer code; MeansKeyCreditTransfer (code 30) is rejected.
			Key:            pay.MeansKeyDebitTransfer,
			CreditTransfer: []*pay.CreditTransfer{{IBAN: "DK5000400440116243", BIC: "DABADKKK"}},
		},
	}
}

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

func TestNormalizeDKBankBranch(t *testing.T) {
	t.Run("stamps DK on a means-42 branch", func(t *testing.T) {
		// The branch address only exists to carry the reg. nr. label, but
		// EN 16931 requires a country on every address (BR-9 et al.).
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key: pay.MeansKeyOther,
				Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "42"}),
				CreditTransfer: []*pay.CreditTransfer{
					{Number: "0440116243", Branch: &org.Address{Label: "1234"}},
				},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.Equal(t, "DK", inv.Payment.Instructions.CreditTransfer[0].Branch.Country.String())
	})

	t.Run("leaves other means untouched", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = bankPayment()
		inv.Payment.Instructions.CreditTransfer[0].Branch = &org.Address{Label: "1234"}
		require.NoError(t, inv.Calculate())
		assert.Empty(t, inv.Payment.Instructions.CreditTransfer[0].Branch.Country)
	})
}

func TestNormalizePartyParticipant(t *testing.T) {
	t.Run("DK party without participant derives the CVR endpoint", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Inboxes = nil
		inv.Supplier.Endpoints = nil
		inv.Customer.Inboxes = nil
		inv.Customer.Endpoints = nil
		inv.Payment = bankPayment()
		require.NoError(t, inv.Calculate())
		require.Len(t, inv.Supplier.Endpoints, 1)
		assert.Equal(t, "DK:CVR:12345674", inv.Supplier.Endpoints[0].URI.String())
		assert.Empty(t, inv.Supplier.Inboxes, "no Peppol endpoint URI is fabricated; the deprecated inbox is not used")
		require.NoError(t, rules.Validate(inv), "a bare DK party should validate via the derived participant")
	})

	t.Run("an explicit inbox is migrated to an endpoint", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Endpoints = nil
		inv.Supplier.Inboxes = []*org.Inbox{{Scheme: "DK:SE", Code: "12345678"}}
		require.NoError(t, inv.Calculate())
		assert.Empty(t, inv.Supplier.Inboxes, "the deprecated inbox is migrated away")
		require.Len(t, inv.Supplier.Endpoints, 1, "the inbox becomes the participant endpoint")
		assert.Equal(t, "DK:SE:12345678", inv.Supplier.Endpoints[0].URI.String(),
			"an explicit DK:SE participant wins over the derived CVR")
	})

	t.Run("foreign party is left untouched", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Customer.TaxID = &tax.Identity{Country: "DE", Code: "129273398"}
		inv.Customer.Inboxes = nil
		inv.Customer.Endpoints = nil
		require.NoError(t, inv.Calculate())
		assert.Empty(t, inv.Customer.Endpoints, "no participant can be derived for non-DK parties")
	})
}
