package oioubl_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	oioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/invopop/gobl/bill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func creditNote(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(getParsePath(), "credit-note.xml"))
	require.NoError(t, err)
	return string(data)
}

func TestParsePrecedingUUIDPairsByCode(t *testing.T) {
	// The generic parser skips a billing reference it cannot resolve, so index
	// pairing would put this UUID on whichever document slid into its place.
	skew := `<cac:BillingReference>
    <cac:BillingReferenceLine><cbc:ID>1</cbc:ID></cac:BillingReferenceLine>
  </cac:BillingReference>
  <cac:BillingReference>`
	doc := strings.Replace(creditNote(t), "<cac:BillingReference>", skew, 1)

	in, err := oioubl.ParseInvoice([]byte(doc))
	require.NoError(t, err)
	env, err := in.Convert()
	require.NoError(t, err)
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)

	require.Len(t, inv.Preceding, 1)
	assert.Equal(t, "A00095678", inv.Preceding[0].Code.String())
	assert.Equal(t, "9756b4d0-8815-1029-857a-e388fe63f399", inv.Preceding[0].UUID.String())
}

func TestParseContractReferenceUUID(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(getParsePath(), "used-invoice_real.xml"))
	require.NoError(t, err)

	in, err := oioubl.ParseInvoice(data)
	require.NoError(t, err)
	env, err := in.Convert()
	require.NoError(t, err)
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)

	require.NotNil(t, inv.Ordering)
	require.NotEmpty(t, inv.Ordering.Contracts)
	assert.Equal(t, "234", inv.Ordering.Contracts[0].Code.String())
	assert.Equal(t, "6e09886b-dc6e-439f-82d1-7ccac7f4e3b1", inv.Ordering.Contracts[0].UUID.String())
}

