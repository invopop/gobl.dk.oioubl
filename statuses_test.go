package oioubl_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invopop/gobl"
	oioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/invopop/gobl.dk.oioubl/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testStatus builds a valid response status: the customer refuses an invoice,
// so the parties keep their original transaction roles. Endpoints and legal
// identities are left for the addon to derive from the Danish tax IDs.
func testStatus() *bill.Status {
	return &bill.Status{
		Regime:    tax.WithRegime("DK"),
		Addons:    tax.WithAddons(oioubl.Addons...),
		Type:      bill.StatusTypeResponse,
		Code:      "RESP001",
		IssueDate: cal.MakeDate(2026, 1, 15),
		Supplier: &org.Party{
			Name:  "Eksempel A/S",
			TaxID: &tax.Identity{Country: "DK", Code: "12345674"},
		},
		Customer: &org.Party{
			Name:  "Kunde ApS",
			TaxID: &tax.Identity{Country: "DK", Code: "88146328"},
		},
		Lines: []*bill.StatusLine{
			{
				Key: bill.StatusLineRejected,
				Doc: &org.DocumentRef{Code: "INV1000"},
			},
		},
	}
}

func convertStatus(t *testing.T, st *bill.Status) *oioubl.ApplicationResponse {
	t.Helper()
	env, err := gobl.Envelop(st)
	require.NoError(t, err)
	doc, err := oioubl.Convert(env)
	require.NoError(t, err)
	ar, ok := doc.(*oioubl.ApplicationResponse)
	require.True(t, ok, "expected an application response, got %T", doc)
	return ar
}

// TestConvertStatus pins the OIOUBL details the plain conversion leaves empty.
func TestConvertStatus(t *testing.T) {
	ar := convertStatus(t, testStatus())

	assert.Equal(t, "OIOUBL-2.1", ar.CustomizationID)
	require.NotNil(t, ar.ProfileID)
	assert.Equal(t, oioubl.ProfileID, ar.ProfileID.Value)
	require.NotNil(t, ar.ProfileID.SchemeID)
	assert.Equal(t, "urn:oioubl:id:profileid-1.2", *ar.ProfileID.SchemeID)

	// The customer answers, so it sends; the supplier is told.
	require.NotNil(t, ar.SenderParty)
	require.NotNil(t, ar.SenderParty.EndpointID)
	assert.Equal(t, "DK88146328", ar.SenderParty.EndpointID.Value)
	assert.Equal(t, "DK:CVR", ar.SenderParty.EndpointID.SchemeID)
	require.NotNil(t, ar.SenderParty.PartyLegalEntity, "sender requires a PartyLegalEntity (F-APR040)")
	require.NotNil(t, ar.ReceiverParty)
	require.NotNil(t, ar.ReceiverParty.EndpointID)
	assert.Equal(t, "DK12345674", ar.ReceiverParty.EndpointID.Value)

	require.Len(t, ar.DocumentResponse, 1)
	dr := ar.DocumentResponse[0]
	require.NotNil(t, dr.Response)
	require.NotNil(t, dr.Response.ResponseCode)
	assert.Equal(t, "BusinessReject", dr.Response.ResponseCode.Value)
	require.NotNil(t, dr.Response.ResponseCode.ListID)
	assert.Equal(t, "urn:oioubl:codelist:responsecode-1.1", *dr.Response.ResponseCode.ListID)
	assert.Equal(t, "1", dr.Response.ReferenceID)
	require.NotNil(t, dr.DocumentReference)
	assert.Equal(t, "INV1000", dr.DocumentReference.ID)
	require.NotNil(t, dr.DocumentReference.DocumentTypeCode)
	assert.Equal(t, "Invoice", dr.DocumentReference.DocumentTypeCode.Value)
}

// TestConvertStatusCodes pins the wire code and profile each status key
// produces, and that the extension overrides the key.
func TestConvertStatusCodes(t *testing.T) {
	tests := []struct {
		name        string
		key         cbc.Key
		ext         cbc.Code
		wantCode    string
		wantProfile string
	}{
		{name: "accepted", key: bill.StatusLineAccepted, wantCode: "BusinessAccept", wantProfile: oioubl.ProfileID},
		{name: "rejected", key: bill.StatusLineRejected, wantCode: "BusinessReject", wantProfile: oioubl.ProfileID},
		{name: "acknowledged", key: bill.StatusLineAcknowledged, wantCode: "ProfileAccept", wantProfile: oioubl.ProfileID},
		{name: "error", key: bill.StatusLineError, wantCode: "TechnicalReject", wantProfile: "NONE"},
		{name: "extension overrides the key", key: bill.StatusLineError, ext: "ProfileReject", wantCode: "ProfileReject", wantProfile: "NONE"},
		{name: "technical accept names its own profile", key: bill.StatusLineAcknowledged, ext: "TechnicalAccept", wantCode: "TechnicalAccept", wantProfile: "Procurement-TecRes-1.0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			st := testStatus()
			st.Lines[0].Key = test.key
			if test.ext != "" {
				st.Lines[0].Ext = tax.ExtensionsOf(cbc.CodeMap{addon.ExtKeyResponseCode: test.ext})
			}
			ar := convertStatus(t, st)
			require.Len(t, ar.DocumentResponse, 1)
			require.NotNil(t, ar.DocumentResponse[0].Response.ResponseCode)
			assert.Equal(t, test.wantCode, ar.DocumentResponse[0].Response.ResponseCode.Value)
			require.NotNil(t, ar.ProfileID)
			assert.Equal(t, test.wantProfile, ar.ProfileID.Value)
		})
	}
}

// TestParseApplicationResponse runs every fixture under
// test/data/parse/responses against its golden GOBL output.
func TestParseApplicationResponse(t *testing.T) {
	responsePath := filepath.Join(getParsePath(), "responses")
	fixtures, err := filepath.Glob(filepath.Join(responsePath, "*.xml"))
	require.NoError(t, err)
	require.NotEmpty(t, fixtures, "no response fixtures found")

	for _, fixture := range fixtures {
		inName := filepath.Base(fixture)
		t.Run(inName, func(t *testing.T) {
			data, err := os.ReadFile(fixture)
			require.NoError(t, err)

			doc, err := oioubl.Parse(data)
			require.NoError(t, err)
			ar, ok := doc.(*oioubl.ApplicationResponse)
			require.True(t, ok, "expected an application response, got %T", doc)

			env, err := ar.Convert()
			require.NoError(t, err)

			out, err := json.MarshalIndent(env.Extract(), "", "\t")
			require.NoError(t, err)
			out = append(out, '\n')

			outPath := filepath.Join(responsePath, "out", strings.Replace(inName, ".xml", ".json", 1))
			if *update {
				require.NoError(t, os.WriteFile(outPath, out, 0644))
			}
			expected, err := os.ReadFile(outPath)
			assert.NoError(t, err)
			assert.JSONEq(t, string(expected), string(out), "Output should match the expected GOBL JSON. Update with --update flag.")
		})
	}
}

// TestParseResponseCodes pins how each of OIOUBL's six response codes lands on
// GOBL's four status keys, with the wire code kept in the extension.
func TestParseResponseCodes(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join(getParsePath(), "responses", "application-response.xml"))
	require.NoError(t, err)

	tests := []struct {
		code    string
		wantKey cbc.Key
	}{
		{code: "BusinessAccept", wantKey: bill.StatusLineAccepted},
		{code: "BusinessReject", wantKey: bill.StatusLineRejected},
		{code: "ProfileAccept", wantKey: bill.StatusLineAcknowledged},
		{code: "TechnicalAccept", wantKey: bill.StatusLineAcknowledged},
		{code: "ProfileReject", wantKey: bill.StatusLineError},
		{code: "TechnicalReject", wantKey: bill.StatusLineError},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			data := strings.Replace(string(fixture), "BusinessReject", test.code, 1)
			doc, err := oioubl.Parse([]byte(data))
			require.NoError(t, err)
			ar, ok := doc.(*oioubl.ApplicationResponse)
			require.True(t, ok)

			env, err := ar.Convert()
			require.NoError(t, err)
			st, ok := env.Extract().(*bill.Status)
			require.True(t, ok)
			require.Len(t, st.Lines, 1)
			assert.Equal(t, test.wantKey, st.Lines[0].Key)
			assert.Equal(t, test.code, st.Lines[0].Ext.Get(addon.ExtKeyResponseCode).String())
		})
	}

	t.Run("unknown code is refused", func(t *testing.T) {
		data := strings.Replace(string(fixture), "BusinessReject", "MaybeLater", 1)
		doc, err := oioubl.Parse([]byte(data))
		require.NoError(t, err)
		ar, ok := doc.(*oioubl.ApplicationResponse)
		require.True(t, ok)

		_, err = ar.Convert()
		assert.ErrorContains(t, err, "unknown OIOUBL response code")
	})
}
