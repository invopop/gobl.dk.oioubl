package oioubl_test

import (
	"os"
	"path/filepath"
	"testing"

	oioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractBinaryAttachmentsFromContractReference(t *testing.T) {
	// OIOUBL's own samples embed the contract under ContractDocumentReference,
	// which the base's scan of AdditionalDocumentReference never reaches.
	data, err := os.ReadFile(filepath.Join(getParsePath(), "used-invoice_real.xml"))
	require.NoError(t, err)

	in, err := oioubl.ParseInvoice(data)
	require.NoError(t, err)

	atts := in.ExtractBinaryAttachments()
	require.Len(t, atts, 1)
	assert.Equal(t, "234", atts[0].ID)
	assert.Equal(t, "image/tiff", atts[0].MimeCode)
	assert.NotEmpty(t, atts[0].Data)
}
