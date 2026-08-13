package oioubl_test

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	oioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExciseBaseSurvivesRoundTrip pins the taxable base of a flat-rate duty
// across XML -> GOBL -> XML -> GOBL. The document-level charge keeps base and
// percent only because they reproduce the amount exactly; a progressive duty
// like the registration tax keeps its amount alone, since GOBL recomputes a
// charge's amount whenever a percent is present.
func TestExciseBaseSurvivesRoundTrip(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(getParsePath(), "excise-line-and-document.xml"))
	require.NoError(t, err)

	in, err := oioubl.ParseInvoice(data)
	require.NoError(t, err)
	env, err := in.Convert()
	require.NoError(t, err)

	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)
	require.Len(t, inv.Charges, 1)
	require.NotNil(t, inv.Charges[0].Base, "a flat-rate duty keeps its taxable base")
	assert.Equal(t, "200000.00", inv.Charges[0].Base.String())
	assert.Equal(t, "100000.00", inv.Charges[0].Amount.String(), "the wire amount stays authoritative")

	// The second pass only sees what the first one emits, so the base has to
	// be written back to the wire to survive.
	doc, err := oioubl.ConvertInvoice(env)
	require.NoError(t, err)
	out, err := xml.Marshal(doc)
	require.NoError(t, err)
	in2, err := oioubl.ParseInvoice(out)
	require.NoError(t, err)
	env2, err := in2.Convert()
	require.NoError(t, err)
	inv2, ok := env2.Extract().(*bill.Invoice)
	require.True(t, ok)
	require.Len(t, inv2.Charges, 1)
	require.NotNil(t, inv2.Charges[0].Base)
	assert.Equal(t, "200000.00", inv2.Charges[0].Base.String())
}

func TestParseExciseMirrorMatchesNumerically(t *testing.T) {
	// A document-level restatement written with fewer decimals is still the
	// same duty; a string comparison would double-count it.
	data, err := os.ReadFile(filepath.Join(getParsePath(), "excise-line-and-document.xml"))
	require.NoError(t, err)
	// The document-level duty block is the one-tab-indented one; the line's
	// own copy is nested deeper and stays as written.
	doc := strings.Replace(string(data),
		"\t<cac:TaxTotal>\n\t\t<cbc:TaxAmount currencyID=\"DKK\">100000.00</cbc:TaxAmount>\n\t\t<cac:TaxSubtotal>\n\t\t\t<cbc:TaxableAmount currencyID=\"DKK\">200000.00</cbc:TaxableAmount>\n\t\t\t<cbc:TaxAmount currencyID =\"DKK\">100000.00</cbc:TaxAmount>",
		"\t<cac:TaxTotal>\n\t\t<cbc:TaxAmount currencyID=\"DKK\">100000.0</cbc:TaxAmount>\n\t\t<cac:TaxSubtotal>\n\t\t\t<cbc:TaxableAmount currencyID=\"DKK\">200000.0</cbc:TaxableAmount>\n\t\t\t<cbc:TaxAmount currencyID =\"DKK\">100000.0</cbc:TaxAmount>", 1)
	require.NotEqual(t, string(data), doc, "the fixture's document-level duty should have been rewritten")

	in, err := oioubl.ParseInvoice([]byte(doc))
	require.NoError(t, err)
	env, err := in.Convert()
	require.NoError(t, err)
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)

	require.Len(t, inv.Charges, 1, "the reformatted mirror is still the same duty")
	assert.Empty(t, inv.Lines[0].Charges)
}

// TestParseExciseRestatedAtBothLevels covers an official sample whose duty
// appears on the line and again in the document total. Only the document-level
// charge can state the VAT the duty is levied at, so that is the one to keep --
// charging it on the line instead taxes the duty at the line's own rate.
func TestParseExciseRestatedAtBothLevels(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(getParsePath(), "excise-line-and-document.xml"))
	require.NoError(t, err)

	in, err := oioubl.ParseInvoice(data)
	require.NoError(t, err)
	env, err := in.Convert()
	require.NoError(t, err)
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)

	require.Len(t, inv.Lines, 1)
	assert.Empty(t, inv.Lines[0].Charges, "the duty belongs at document level, not on the line")

	require.Len(t, inv.Charges, 1)
	duty := inv.Charges[0]
	assert.Equal(t, "100000.00", duty.Amount.String())
	require.Len(t, duty.Taxes, 1)
	assert.Equal(t, tax.KeyZero, duty.Taxes[0].Key, "the duty states its own VAT type")

	// The line's own category is the one levied on the whole line, not the
	// first one the document happens to list.
	require.Len(t, inv.Lines[0].Taxes, 1)
	assert.Equal(t, tax.KeyStandard, inv.Lines[0].Taxes[0].Key)

	// The wire states 350000.00; anything else means VAT landed on the duty.
	assert.Equal(t, "350000.00", inv.Totals.Payable.String())
}
