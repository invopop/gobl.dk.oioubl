package dkoioubl_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invopop/gobl"
	dkoioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/pay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseXMLInvoice parses an XML fixture from test/data/parse into a GOBL envelope.
func parseXMLInvoice(t *testing.T, name string) *gobl.Envelope {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(getParsePath(), name))
	require.NoError(t, err)
	doc, err := dkoioubl.Parse(data)
	require.NoError(t, err)
	inv, ok := doc.(*dkoioubl.Invoice)
	require.True(t, ok)
	env, err := inv.Convert()
	require.NoError(t, err)
	return env
}

func TestParseDueDateAndNestedBIC(t *testing.T) {
	// OIOUBL moves the invoice due date onto the payment means (clearing the
	// root) and nests the BIC under FinancialInstitution/ID after stripping the
	// branch ID (F-LIB295). Both must survive the parse.
	env := parseXMLInvoice(t, "invoice-bare.xml")
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)
	require.NotNil(t, inv.Payment)

	require.NotNil(t, inv.Payment.Terms)
	require.Len(t, inv.Payment.Terms.DueDates, 1)
	require.NotNil(t, inv.Payment.Terms.DueDates[0].Date)
	assert.Equal(t, "2024-06-15", inv.Payment.Terms.DueDates[0].Date.String())

	require.NotNil(t, inv.Payment.Instructions)
	require.Len(t, inv.Payment.Instructions.CreditTransfer, 1)
	assert.Equal(t, "DABADKKK", inv.Payment.Instructions.CreditTransfer[0].BIC)
}

func TestParseNemKonto(t *testing.T) {
	// NemKonto (97) carries no account or reference details at all; the key is
	// pinned to "other" so the explicit means code survives EN 16931
	// normalization.
	env := parseXMLInvoice(t, "nemkonto.xml")
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)
	require.NotNil(t, inv.Payment)

	instr := inv.Payment.Instructions
	require.NotNil(t, instr)
	assert.Equal(t, pay.MeansKeyOther, instr.Key)
	assert.Equal(t, "97", instr.Ext.Get(untdid.ExtKeyPaymentMeans).String())
	assert.Empty(t, instr.CreditTransfer)
}
