package dkoioubl_test

import (
	"path/filepath"
	"testing"

	dkoioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A URL attachment converts to a cac:AdditionalDocumentReference holding an
// external reference (a link, not the bytes), stamped with the OIOUBL
// DocumentType every reference requires (F-LIB092).
func TestConvertAttachmentEmitsExternalReference(t *testing.T) {
	env := loadTestEnvelope(t, filepath.Join(getConvertPath(), "happy-path_real.json"))
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)
	inv.Attachments = []*org.Attachment{{
		Code: "DOC-1",
		Name: "spec.pdf",
		URL:  "https://example.dk/spec.pdf",
		MIME: "application/pdf",
	}}

	doc, err := dkoioubl.ConvertInvoice(env)
	require.NoError(t, err)
	data, err := dkoioubl.Bytes(doc)
	require.NoError(t, err)

	out := string(data)
	assert.Contains(t, out, "<cac:AdditionalDocumentReference>")
	assert.Contains(t, out, "<cbc:URI>https://example.dk/spec.pdf</cbc:URI>")
	assert.Contains(t, out, "Supporting Document")
}

// gov-dk's inbound pipeline pulls embedded documents off a received invoice via
// ExtractBinaryAttachments; the base64 payload is decoded and the metadata kept.
func TestParseExtractsBinaryAttachment(t *testing.T) {
	const doc = `<?xml version="1.0" encoding="UTF-8"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2" xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2" xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2">
  <cbc:ID>TEST-1</cbc:ID>
  <cac:AdditionalDocumentReference>
    <cbc:ID>DOC-1</cbc:ID>
    <cbc:DocumentDescription>Spec</cbc:DocumentDescription>
    <cac:Attachment>
      <cbc:EmbeddedDocumentBinaryObject mimeCode="application/pdf" filename="spec.pdf">aGVsbG8=</cbc:EmbeddedDocumentBinaryObject>
    </cac:Attachment>
  </cac:AdditionalDocumentReference>
</Invoice>`

	parsed, err := dkoioubl.Parse([]byte(doc))
	require.NoError(t, err)
	inv, ok := parsed.(*dkoioubl.Invoice)
	require.True(t, ok)

	atts := inv.ExtractBinaryAttachments()
	require.Len(t, atts, 1)
	assert.Equal(t, "DOC-1", atts[0].ID)
	assert.Equal(t, "spec.pdf", atts[0].Filename)
	assert.Equal(t, "application/pdf", atts[0].MimeCode)
	assert.Equal(t, []byte("hello"), atts[0].Data)
}
