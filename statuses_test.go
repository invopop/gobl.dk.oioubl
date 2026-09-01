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
	ar, err := oioubl.ConvertApplicationResponse(env)
	require.NoError(t, err)
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
	require.NotNil(t, dr.Response.ResponseCode.ListAgencyID)
	assert.Equal(t, "320", *dr.Response.ResponseCode.ListAgencyID)
	assert.Equal(t, "1", dr.Response.ReferenceID)
	require.NotNil(t, dr.DocumentReference)
	assert.Equal(t, "INV1000", dr.DocumentReference.ID)
	require.NotNil(t, dr.DocumentReference.DocumentTypeCode)
	assert.Equal(t, "Invoice", dr.DocumentReference.DocumentTypeCode.Value)
	require.NotNil(t, dr.DocumentReference.DocumentTypeCode.ListID)
	assert.Equal(t, "urn:oioubl:codelist:responsedocumenttypecode-1.1", *dr.DocumentReference.DocumentTypeCode.ListID)
	require.NotNil(t, dr.DocumentReference.DocumentTypeCode.ListAgencyID)
	assert.Equal(t, "320", *dr.DocumentReference.DocumentTypeCode.ListAgencyID)
}

// TestConvertStatusCreditNote names the referenced document a CreditNote.
func TestConvertStatusCreditNote(t *testing.T) {
	st := testStatus()
	st.Lines[0].Doc.Type = bill.InvoiceTypeCreditNote
	ar := convertStatus(t, st)
	require.Len(t, ar.DocumentResponse, 1)
	require.NotNil(t, ar.DocumentResponse[0].DocumentReference)
	require.NotNil(t, ar.DocumentResponse[0].DocumentReference.DocumentTypeCode)
	assert.Equal(t, "CreditNote", ar.DocumentResponse[0].DocumentReference.DocumentTypeCode.Value)
}

// TestConvertStatusAddsAddon converts a status that never declared the addon:
// Convert adds it, so its normalizations and rules still run.
func TestConvertStatusAddsAddon(t *testing.T) {
	st := testStatus()
	st.Addons = tax.Addons{}
	ar := convertStatus(t, st)
	require.Len(t, ar.DocumentResponse, 1)
	require.NotNil(t, ar.DocumentResponse[0].Response.ResponseCode)
	assert.Equal(t, "BusinessReject", ar.DocumentResponse[0].Response.ResponseCode.Value)
	require.NotNil(t, ar.SenderParty.EndpointID, "the addon's derived endpoint proves normalization ran")
}

// TestConvertStatusRefusals pins what Convert refuses rather than converts.
func TestConvertStatusRefusals(t *testing.T) {
	t.Run("non-response status type", func(t *testing.T) {
		st := testStatus()
		st.Type = bill.StatusTypeSystem
		env, err := gobl.Envelop(st)
		require.NoError(t, err)
		_, err = oioubl.Convert(env)
		assert.ErrorIs(t, err, oioubl.ErrUnsupportedDocumentType)
	})

	t.Run("missing customer", func(t *testing.T) {
		st := testStatus()
		st.Customer = nil
		env, err := gobl.Envelop(st)
		require.NoError(t, err)
		_, err = oioubl.Convert(env)
		assert.ErrorContains(t, err, "customer is required")
	})

	t.Run("response code extension contradicting the key", func(t *testing.T) {
		st := testStatus()
		st.Lines[0].Key = bill.StatusLineAccepted
		st.Lines[0].Ext = tax.ExtensionsOf(cbc.CodeMap{addon.ExtKeyResponseCode: "BusinessReject"})
		env, err := gobl.Envelop(st)
		require.NoError(t, err)
		_, err = oioubl.Convert(env)
		assert.ErrorContains(t, err, "must agree with the status key")
	})

	t.Run("error key is not an OIOUBL response event", func(t *testing.T) {
		// Every negative OIOUBL answer is a definitive rejection; a status
		// keyed error has no honest wire code and is refused.
		st := testStatus()
		st.Lines[0].Key = bill.StatusLineError
		env, err := gobl.Envelop(st)
		require.NoError(t, err)
		_, err = oioubl.Convert(env)
		assert.ErrorContains(t, err, "must be one OIOUBL supports")
	})

	t.Run("response code outside the codelist", func(t *testing.T) {
		st := testStatus()
		st.Lines[0].Ext = tax.ExtensionsOf(cbc.CodeMap{addon.ExtKeyResponseCode: "BusinesReject"})
		env, err := gobl.Envelop(st)
		require.NoError(t, err)
		_, err = oioubl.Convert(env)
		assert.Error(t, err, "a typo'd code must not reach the wire")
	})

	t.Run("customer without a legal identity source (F-APR040)", func(t *testing.T) {
		st := testStatus()
		st.Customer = &org.Party{
			Name:      "Beispiel GmbH",
			TaxID:     &tax.Identity{Country: "DE", Code: "111111125"},
			Endpoints: []*org.Endpoint{{URI: "GLN:4035811991021"}},
		}
		env, err := gobl.Envelop(st)
		require.NoError(t, err)
		_, err = oioubl.Convert(env)
		assert.ErrorContains(t, err, "F-APR040")
	})

	t.Run("non-bill document", func(t *testing.T) {
		env, err := gobl.Envelop(&org.Party{Name: "Eksempel A/S"})
		require.NoError(t, err)
		_, err = oioubl.Convert(env)
		assert.ErrorIs(t, err, oioubl.ErrUnsupportedDocumentType)
	})

	t.Run("invoice envelope through ConvertApplicationResponse", func(t *testing.T) {
		env := loadTestEnvelope(t, filepath.Join(getConvertPath(), "dk-bank.json"))
		_, err := oioubl.ConvertApplicationResponse(env)
		assert.ErrorContains(t, err, "expected application response")
	})

	t.Run("invoice XML through ParseApplicationResponse", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(getParsePath(), "invoice-bare.xml"))
		require.NoError(t, err)
		_, err = oioubl.ParseApplicationResponse(data)
		assert.ErrorContains(t, err, "expected application response")
	})
}

// TestConvertStatusEndpointSelection pins that the wire endpoint is the
// OIOUBL-register one, not whichever endpoint happens to come first.
func TestConvertStatusEndpointSelection(t *testing.T) {
	st := testStatus()
	st.Customer.Endpoints = []*org.Endpoint{
		{URI: "iso6523-actorid-upis::0184:88146328"},
		{URI: "GLN:5798009883735"},
	}
	ar := convertStatus(t, st)
	require.NotNil(t, ar.SenderParty.EndpointID)
	assert.Equal(t, "GLN", ar.SenderParty.EndpointID.SchemeID)
	assert.Equal(t, "5798009883735", ar.SenderParty.EndpointID.Value)
}

// TestStatusRoundTrip converts a status out to OIOUBL and parses it back:
// the wire-only DK prefixes and the code mapping must cancel out.
func TestStatusRoundTrip(t *testing.T) {
	ar := convertStatus(t, testStatus())
	data, err := oioubl.Bytes(ar)
	require.NoError(t, err)

	parsed, err := oioubl.ParseApplicationResponse(data)
	require.NoError(t, err)
	env, err := parsed.Convert()
	require.NoError(t, err)
	st, ok := env.Extract().(*bill.Status)
	require.True(t, ok)

	assert.Equal(t, bill.StatusTypeResponse, st.Type)
	assert.Equal(t, cbc.Code("RESP001"), st.Code)
	require.Len(t, st.Lines, 1)
	assert.Equal(t, bill.StatusLineRejected, st.Lines[0].Key)
	assert.Equal(t, cbc.Code("BusinessReject"), st.Lines[0].Ext.Get(addon.ExtKeyResponseCode))
	require.NotNil(t, st.Lines[0].Doc)
	assert.Equal(t, cbc.Code("INV1000"), st.Lines[0].Doc.Code)
	require.NotNil(t, st.Supplier.TaxID)
	assert.Equal(t, cbc.Code("12345674"), st.Supplier.TaxID.Code, "the wire-only DK prefix must not survive the round trip")
	require.NotNil(t, st.Customer.TaxID)
	assert.Equal(t, cbc.Code("88146328"), st.Customer.TaxID.Code)
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
		wantScheme  string
	}{
		{name: "accepted", key: bill.StatusLineAccepted, wantCode: "BusinessAccept", wantProfile: oioubl.ProfileID, wantScheme: "urn:oioubl:id:profileid-1.2"},
		{name: "rejected", key: bill.StatusLineRejected, wantCode: "BusinessReject", wantProfile: oioubl.ProfileID, wantScheme: "urn:oioubl:id:profileid-1.2"},
		{name: "acknowledged", key: bill.StatusLineAcknowledged, wantCode: "ProfileAccept", wantProfile: oioubl.ProfileID, wantScheme: "urn:oioubl:id:profileid-1.2"},
		{name: "technical reject via the extension", key: bill.StatusLineRejected, ext: "TechnicalReject", wantCode: "TechnicalReject", wantProfile: "NONE", wantScheme: "urn:oioubl:id:profileid-1.2"},
		{name: "profile reject via the extension", key: bill.StatusLineRejected, ext: "ProfileReject", wantCode: "ProfileReject", wantProfile: "NONE", wantScheme: "urn:oioubl:id:profileid-1.2"},
		// TecRes only exists from profileid-1.4 on (F-LIB302).
		{name: "technical accept names its own profile", key: bill.StatusLineAcknowledged, ext: "TechnicalAccept", wantCode: "TechnicalAccept", wantProfile: "Procurement-TecRes-1.0", wantScheme: "urn:oioubl:id:profileid-1.6"},
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
			require.NotNil(t, ar.ProfileID.SchemeID)
			assert.Equal(t, test.wantScheme, *ar.ProfileID.SchemeID)
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

			ar, err := oioubl.ParseApplicationResponse(data)
			require.NoError(t, err)

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
		{code: "ProfileReject", wantKey: bill.StatusLineRejected},
		{code: "TechnicalReject", wantKey: bill.StatusLineRejected},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			data := strings.Replace(string(fixture), "BusinessReject", test.code, 1)
			ar, err := oioubl.ParseApplicationResponse([]byte(data))
			require.NoError(t, err)

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
		ar, err := oioubl.ParseApplicationResponse([]byte(data))
		require.NoError(t, err)

		_, err = ar.Convert()
		assert.ErrorContains(t, err, "unknown OIOUBL response code")
	})

	t.Run("missing response code is refused", func(t *testing.T) {
		data := strings.Replace(string(fixture),
			`<cbc:ResponseCode listAgencyID="320" listID="urn:oioubl:codelist:responsecode-1.1">BusinessReject</cbc:ResponseCode>`, "", 1)
		ar, err := oioubl.ParseApplicationResponse([]byte(data))
		require.NoError(t, err)

		_, err = ar.Convert()
		assert.ErrorContains(t, err, "carries no response code")
	})

	t.Run("response failing the addon's rules is refused", func(t *testing.T) {
		data := strings.Replace(string(fixture),
			`<cbc:EndpointID schemeAgencyID="9" schemeID="GLN">5798009811578</cbc:EndpointID>`, "", 1)
		ar, err := oioubl.ParseApplicationResponse([]byte(data))
		require.NoError(t, err)

		_, err = ar.Convert()
		assert.ErrorContains(t, err, "endpoint is required")
	})

	t.Run("unsupported customization id is refused", func(t *testing.T) {
		data := strings.Replace(string(fixture), "OIOUBL-2.01",
			"urn:fdc:peppol.eu:poacc:trns:invoice_response:3", 1)
		ar, err := oioubl.ParseApplicationResponse([]byte(data))
		require.NoError(t, err)

		_, err = ar.Convert()
		assert.ErrorIs(t, err, oioubl.ErrUnsupportedCustomizationID)
	})
}
