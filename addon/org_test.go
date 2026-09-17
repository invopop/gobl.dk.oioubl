package addon_test

import (
	"testing"

	"github.com/invopop/gobl.dk.oioubl/addon"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartyValidation(t *testing.T) {

	// The DK regime, not this addon, is what rejects a codeless DK supplier;
	// that is why there is no OIOUBL rule for F-INV031. If this ever stops
	// failing, the addon needs its own check back.
	t.Run("supplier without participant or tax ID code fails via the DK regime", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Inboxes = nil
		inv.Supplier.TaxID = &tax.Identity{Country: "DK"}
		require.NoError(t, inv.Calculate())
		// Assert the fault code, not its prose: the regime's wording tracks
		// which identity types it accepts and changes as they are added.
		assert.ErrorContains(t, rules.Validate(inv), "GOBL-DK-BILL-INVOICE-01")
	})

	t.Run("customer without participant or tax ID code fails (F-INV044)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Customer.Inboxes = nil
		inv.Customer.TaxID = &tax.Identity{Country: "DK"}
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv)
		assert.ErrorContains(t, err, "F-INV044")
	})

	t.Run("customer's blank company ID is rejected", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Customer.TaxID = &tax.Identity{Country: "DE", Code: "282741168"}
		inv.Customer.Identities = []*org.Identity{{Scope: org.IdentityScopeLegal, Code: ""}}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "identity code must be provided")
	})

	t.Run("Danish customer needs no explicit legal identity (CVR fabricated)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
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
		assert.Equal(t, "nemhandel:dk:cvr:12345674", inv.Supplier.Endpoints[0].URI.String())
		assert.Empty(t, inv.Supplier.Inboxes, "no Peppol endpoint URI is fabricated; the deprecated inbox is not used")
		require.NoError(t, rules.Validate(inv), "a bare DK party should validate via the derived participant")
	})

	// Documents stored before the network scheme existed still read, and
	// normalizing settles them on the current spelling.
	t.Run("a bare endpoint is rewritten onto the network scheme", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Inboxes = nil
		inv.Supplier.Endpoints = []*org.Endpoint{{URI: "DK:SE:12345678"}}
		require.NoError(t, inv.Calculate())
		require.Len(t, inv.Supplier.Endpoints, 1, "no second endpoint is derived from the tax ID")
		assert.Equal(t, "nemhandel:dk:se:12345678", inv.Supplier.Endpoints[0].URI.String())
	})

	// The converter adds the DK prefix in the XML (F-LIB180 for CVR), so the
	// stored code does without it and one address has one spelling.
	t.Run("a CVR or SE endpoint is stored without the DK prefix", func(t *testing.T) {
		for given, want := range map[string]string{
			"DK:CVR:12345674":             "nemhandel:dk:cvr:12345674",
			"DK:CVR:DK12345674":           "nemhandel:dk:cvr:12345674",
			"nemhandel:DK:CVR:DK12345674": "nemhandel:dk:cvr:12345674",
			"DK:SE:12345678":              "nemhandel:dk:se:12345678",
			"DK:SE:DK12345678":            "nemhandel:dk:se:12345678",
		} {
			inv := testInvoiceStandard(t)
			inv.Supplier.Inboxes = nil
			inv.Supplier.Endpoints = []*org.Endpoint{{URI: cbc.URI(given)}}
			require.NoError(t, inv.Calculate())
			require.Len(t, inv.Supplier.Endpoints, 1)
			assert.Equal(t, want, inv.Supplier.Endpoints[0].URI.String(), "given %q", given)
		}
	})

	// A code that is only the prefix is left alone rather than rewritten to
	// an empty one that nothing would recognise.
	t.Run("a prefix-only code is not rewritten", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Inboxes = nil
		inv.Supplier.Endpoints = []*org.Endpoint{{URI: "DK:CVR:DK"}}
		require.NoError(t, inv.Calculate())
		require.Len(t, inv.Supplier.Endpoints, 1)
		assert.Equal(t, "DK:CVR:DK", inv.Supplier.Endpoints[0].URI.String())
	})

	// No other register has a prefix convention, so nothing is invented for
	// GLN or CPR: the code is kept exactly as given.
	t.Run("other registers keep the code they were given", func(t *testing.T) {
		for given, want := range map[string]string{
			"GLN:5798009883735": "nemhandel:gln:5798009883735",
			"DK:CPR:1111111118": "nemhandel:dk:cpr:1111111118",
		} {
			inv := testInvoiceStandard(t)
			inv.Supplier.Inboxes = nil
			inv.Supplier.Endpoints = []*org.Endpoint{{URI: cbc.URI(given)}}
			require.NoError(t, inv.Calculate())
			require.Len(t, inv.Supplier.Endpoints, 1)
			assert.Equal(t, want, inv.Supplier.Endpoints[0].URI.String(), "given %q", given)
		}
	})

	// The register means the same thing in any case, so one spelling is kept.
	t.Run("a register is read regardless of case", func(t *testing.T) {
		for _, given := range []string{
			"nemhandel:dk:cvr:12345674",
			"NEMHANDEL:DK:CVR:12345674",
			"dk:cvr:12345674",
		} {
			inv := testInvoiceStandard(t)
			inv.Supplier.Inboxes = nil
			inv.Supplier.Endpoints = []*org.Endpoint{{URI: cbc.URI(given)}}
			require.NoError(t, inv.Calculate())
			require.Len(t, inv.Supplier.Endpoints, 1, "given %q", given)
			assert.Equal(t, "nemhandel:dk:cvr:12345674", inv.Supplier.Endpoints[0].URI.String(),
				"given %q", given)
		}
	})

	// A shorthand like "cvr:12345674" names no register OIOUBL knows, and
	// "se:12345678" would be ambiguous between DK:SE and SE:ORGNR.
	t.Run("an app's internal shorthand is not a register", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Inboxes = nil
		inv.Supplier.Endpoints = []*org.Endpoint{{URI: "cvr:12345674"}}
		require.NoError(t, inv.Calculate())
		assert.Equal(t, "cvr:12345674", inv.Supplier.Endpoints[0].URI.String(),
			"left untouched: it names no network we recognise")
		require.Len(t, inv.Supplier.Endpoints, 2, "so a Danish endpoint is still derived")
		assert.Equal(t, "nemhandel:dk:cvr:12345674", inv.Supplier.Endpoints[1].URI.String())
	})

	// A party may sit on both networks. Rewriting must not touch the Peppol one.
	t.Run("an endpoint on another network is left alone", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Inboxes = nil
		inv.Supplier.Endpoints = []*org.Endpoint{{URI: "iso6523-actorid-upis::0184:12345674"}}
		require.NoError(t, inv.Calculate())
		require.Len(t, inv.Supplier.Endpoints, 2, "a Danish one is derived alongside")
		assert.Equal(t, "iso6523-actorid-upis::0184:12345674", inv.Supplier.Endpoints[0].URI.String())
		assert.Equal(t, "nemhandel:dk:cvr:12345674", inv.Supplier.Endpoints[1].URI.String())
	})

	// An inbox is settled like an endpoint that arrived as one.
	t.Run("a migrated inbox is normalized too", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Endpoints = nil
		inv.Supplier.Inboxes = []*org.Inbox{{Scheme: "dk:cvr", Code: "DK12345674"}}
		require.NoError(t, inv.Calculate())
		require.Len(t, inv.Supplier.Endpoints, 1)
		assert.Equal(t, "nemhandel:dk:cvr:12345674", inv.Supplier.Endpoints[0].URI.String())
	})

	t.Run("an explicit inbox is migrated to an endpoint", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Endpoints = nil
		inv.Supplier.Inboxes = []*org.Inbox{{Scheme: "DK:SE", Code: "12345678"}}
		require.NoError(t, inv.Calculate())
		assert.Empty(t, inv.Supplier.Inboxes, "the deprecated inbox is migrated away")
		require.Len(t, inv.Supplier.Endpoints, 1, "the inbox becomes the participant endpoint")
		assert.Equal(t, "nemhandel:dk:se:12345678", inv.Supplier.Endpoints[0].URI.String(),
			"an explicit DK:SE participant wins over the derived CVR")
	})

	// EN 16931 allows only one legal-scope identity (BT-30/BT-47), so deriving
	// the CVR unconditionally would break any party that already states it.
	t.Run("a party's own legal identity is not duplicated", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Supplier.Identities = []*org.Identity{
			{Scope: org.IdentityScopeLegal, Code: "12345674"},
		}
		require.NoError(t, inv.Calculate())
		require.Len(t, inv.Supplier.Identities, 1, "the stated legal identity is kept, not duplicated")
		assert.NoError(t, rules.Validate(inv))
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

// SplitEndpointURI is the one place that decides whether an endpoint is a
// NemHandel one, so its edges are pinned: what it accepts, how it spells the
// register back, and what it refuses.
func TestSplitEndpointURI(t *testing.T) {
	for _, tc := range []struct {
		uri      string
		register string
		code     string
		ok       bool
	}{
		{"nemhandel:dk:cvr:12345674", "DK:CVR", "12345674", true},
		{"nemhandel:DK:CVR:12345674", "DK:CVR", "12345674", true},
		{"nemhandel:gln:5798009883735", "GLN", "5798009883735", true},
		{"DK:CVR:12345674", "DK:CVR", "12345674", true},
		{"GLN:5798009883735", "GLN", "5798009883735", true},
		{"iso6523-actorid-upis::0184:12345674", "", "", false},
		{"cvr:12345674", "", "", false},
		{"nemhandel:DK:CVR:", "", "", false},
		{"nemhandel:", "", "", false},
		{"12345674", "", "", false},
		{"", "", "", false},
	} {
		register, code, ok := addon.SplitEndpointURI(cbc.URI(tc.uri))
		assert.Equal(t, tc.ok, ok, "ok for %q", tc.uri)
		assert.Equal(t, tc.register, register.String(), "register for %q", tc.uri)
		assert.Equal(t, tc.code, code.String(), "code for %q", tc.uri)
	}
}

// A nil party has no endpoint, and a nil entry in the list is skipped rather
// than dereferenced. Callers outside a Calculate pass can hand in either.
func TestEndpointNilsAreSkipped(t *testing.T) {
	assert.Nil(t, addon.OIOUBLEndpoint(nil))

	p := &org.Party{Endpoints: []*org.Endpoint{nil, {URI: "nemhandel:dk:cvr:12345674"}}}
	ep := addon.OIOUBLEndpoint(p)
	require.NotNil(t, ep)
	assert.Equal(t, "nemhandel:dk:cvr:12345674", ep.URI.String())
}
