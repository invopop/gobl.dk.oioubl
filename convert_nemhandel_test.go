package oioubl_test

import (
	"testing"

	"github.com/invopop/gobl"
	oioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/invopop/gobl.dk.oioubl/addon"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertNemHandelEndpointURI pins the endpoint URI end to end: however it
// is spelled going in, it is stored as the participant identifier of its
// register and the cbc:EndpointID carries OIOUBL's register name with a
// DK-prefixed CVR or SE code.
func TestConvertNemHandelEndpointURI(t *testing.T) {
	for _, tt := range []struct {
		given, stored, scheme, value string
	}{
		{"DK:CVR:88146328", "iso6523-actorid-upis::0184:88146328", "DK:CVR", "DK88146328"},
		{"DK:CVR:DK88146328", "iso6523-actorid-upis::0184:88146328", "DK:CVR", "DK88146328"},
		{"iso6523-actorid-upis::0184:88146328", "iso6523-actorid-upis::0184:88146328", "DK:CVR", "DK88146328"},
		{"iso6523-actorid-upis::9902:DK88146328", "iso6523-actorid-upis::0184:88146328", "DK:CVR", "DK88146328"},
		{"DK:SE:DK88146328", "iso6523-actorid-upis::0198:DK88146328", "DK:SE", "DK88146328"},
		{"DK:SE:88146328", "iso6523-actorid-upis::0198:DK88146328", "DK:SE", "DK88146328"},
		{"iso6523-actorid-upis::0198:DK88146328", "iso6523-actorid-upis::0198:DK88146328", "DK:SE", "DK88146328"},
	} {
		t.Run(tt.given, func(t *testing.T) {
			env := envelopeWithCustomerEndpoint(t, tt.given)

			// Normalizing must not derive a second endpoint from the tax ID: the
			// party already says where it is addressed.
			out := env.Extract().(*bill.Invoice)
			require.Len(t, out.Customer.Endpoints, 1)
			assert.Equal(t, tt.stored, out.Customer.Endpoints[0].URI.String())

			doc, err := oioubl.ConvertInvoice(env)
			require.NoError(t, err)

			ep := doc.AccountingCustomerParty.Party.EndpointID
			require.NotNil(t, ep)
			assert.Equal(t, tt.scheme, ep.SchemeID)
			assert.Equal(t, tt.value, ep.Value)
		})
	}
}

func envelopeWithCustomerEndpoint(t *testing.T, uri string) *gobl.Envelope {
	t.Helper()
	inv := &bill.Invoice{
		Regime:    tax.WithRegime("DK"),
		Addons:    tax.WithAddons(en16931.V2017, addon.V2),
		IssueDate: cal.MakeDate(2026, 1, 1),
		Type:      "standard",
		Series:    "2026",
		Code:      "1",
		Currency:  "DKK",
		Supplier: &org.Party{
			Name:      "Eksempel A/S",
			TaxID:     &tax.Identity{Country: "DK", Code: "12345674"},
			Addresses: []*org.Address{{Street: "Hovedgaden", Locality: "København", Code: "1000", Country: "DK"}},
		},
		Customer: &org.Party{
			Name:      "Kunde ApS",
			TaxID:     &tax.Identity{Country: "DK", Code: "88146328"},
			Endpoints: []*org.Endpoint{{URI: cbc.URI(uri)}},
			Addresses: []*org.Address{{Street: "Fredericiavej", Locality: "Helsingør", Code: "3000", Country: "DK"}},
		},
		Lines: []*bill.Line{{
			Quantity: num.MakeAmount(1, 0),
			Item:     &org.Item{Name: "vare", Price: num.NewAmount(10000, 2)},
			Taxes:    tax.Set{{Category: "VAT", Percent: num.NewPercentage(25, 2)}},
		}},
	}
	env, err := gobl.Envelop(inv)
	require.NoError(t, err)
	return env
}
