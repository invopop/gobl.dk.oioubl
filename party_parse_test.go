package oioubl_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	oioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bareInvoice reads the minimal fixture so a test can vary one element of an
// otherwise valid document.
func bareInvoice(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(getParsePath(), "invoice-bare.xml"))
	require.NoError(t, err)
	return string(data)
}

func convertString(t *testing.T, doc string) (*bill.Invoice, error) {
	t.Helper()
	in, err := oioubl.ParseInvoice([]byte(doc))
	require.NoError(t, err)
	env, err := in.Convert()
	if err != nil {
		return nil, err
	}
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)
	return inv, nil
}

func TestParseBlankContactFields(t *testing.T) {
	// OIOUBL senders write empty elements freely; the generic parser would read
	// these as a present-but-blank phone and email and fail validation.
	doc := strings.Replace(bareInvoice(t),
		"<cbc:Name>Kontakt Ansvarlig</cbc:Name>",
		"<cbc:Name>Kontakt Ansvarlig</cbc:Name>\n        <cbc:Telephone>   </cbc:Telephone>\n        <cbc:ElectronicMail>\n        </cbc:ElectronicMail>",
		1)
	require.Contains(t, doc, "<cbc:Telephone>")

	inv, err := convertString(t, doc)
	require.NoError(t, err)
	require.NotNil(t, inv.Customer)
	assert.Empty(t, inv.Customer.Telephones, "a whitespace-only telephone is not a telephone")
	assert.Empty(t, inv.Customer.Emails, "a whitespace-only email is not an email")
}

func TestParseBlankContactKeepsRealValues(t *testing.T) {
	doc := strings.Replace(bareInvoice(t),
		"<cbc:Name>Kontakt Ansvarlig</cbc:Name>",
		"<cbc:Name>Kontakt Ansvarlig</cbc:Name>\n        <cbc:ElectronicMail>kontakt@example.dk</cbc:ElectronicMail>",
		1)

	inv, err := convertString(t, doc)
	require.NoError(t, err)
	require.NotNil(t, inv.Customer)
	require.Len(t, inv.Customer.Emails, 1)
	assert.Equal(t, "kontakt@example.dk", inv.Customer.Emails[0].Address)
}

func TestParsePayeeContactID(t *testing.T) {
	// The payee is stripped like the other parties, so its contact id has to
	// come back too or a round trip drops it (F-INV051).
	data, err := os.ReadFile(filepath.Join(getParsePath(), "used-invoice_real.xml"))
	require.NoError(t, err)
	in, err := oioubl.ParseInvoice(data)
	require.NoError(t, err)
	env, err := in.Convert()
	require.NoError(t, err)
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)

	require.NotNil(t, inv.Payment)
	require.NotNil(t, inv.Payment.Payee)
	require.Len(t, inv.Payment.Payee.People, 1)
	require.Len(t, inv.Payment.Payee.People[0].Identities, 1)
	assert.Equal(t, cbc.Code("9000000005"), inv.Payment.Payee.People[0].Identities[0].Code)
}
