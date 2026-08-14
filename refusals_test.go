package oioubl_test

import (
	"strings"
	"testing"

	oioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/stretchr/testify/require"
)

// TestParseRefusesBadInput pins the refusal behaviour for input the converter
// must not touch, so a base-library change cannot start accepting it silently.
func TestParseRefusesBadInput(t *testing.T) {
	order := `<?xml version="1.0" encoding="UTF-8"?>
<Order xmlns="urn:oasis:names:specification:ubl:schema:xsd:Order-2"
  xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
  <cbc:ID>5002701</cbc:ID>
</Order>`
	entity := `<?xml version="1.0"?>
<!DOCTYPE Invoice [ <!ENTITY xxe SYSTEM "file:///etc/hostname"> ]>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
  xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
  <cbc:ID>&xxe;</cbc:ID>
</Invoice>`

	tests := []struct {
		name string
		doc  string
	}{
		{"order document", order},
		{"external entity", entity},
		{"truncated document", bareInvoice(t)[:100]},
		{"not xml", "this is not an invoice"},
		{"empty", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := oioubl.Parse([]byte(tt.doc))
			require.Error(t, err)
		})
	}
}

// TestConvertRefusesForeignProfile pins the customization gate against a
// document that parses fine but belongs to another profile.
func TestConvertRefusesForeignProfile(t *testing.T) {
	doc := strings.Replace(bareInvoice(t), "OIOUBL-2.1", "urn:fdc:peppol.eu:2017:poacc:billing:3.0", 1)
	in, err := oioubl.ParseInvoice([]byte(doc))
	require.NoError(t, err)
	_, err = in.Convert()
	require.Error(t, err)
	require.Contains(t, err.Error(), "customization id")
}
