package oioubl_test

import (
	"os"
	"path/filepath"
	"testing"

	oioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
