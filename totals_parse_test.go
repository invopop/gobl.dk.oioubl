package oioubl_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePayableRounding(t *testing.T) {
	// OIOUBL's afrundingsbeløb; the outbound half already writes it back, so
	// dropping it inbound loses the amount across a round trip.
	doc := strings.Replace(bareInvoice(t),
		`<cbc:PayableAmount currencyID="DKK">41178.80</cbc:PayableAmount>`,
		`<cbc:PayableRoundingAmount currencyID="DKK">0.20</cbc:PayableRoundingAmount>`+
			"\n    "+`<cbc:PayableAmount currencyID="DKK">41179.00</cbc:PayableAmount>`,
		1)
	require.Contains(t, doc, "PayableRoundingAmount")

	inv, err := convertString(t, doc)
	require.NoError(t, err)
	require.NotNil(t, inv.Totals.Rounding, "the wire stated a rounding amount")
	assert.Equal(t, "0.20", inv.Totals.Rounding.String())
	assert.Equal(t, "41179.00", inv.Totals.Payable.String())
}

func TestParseWithoutPayableRounding(t *testing.T) {
	inv, err := convertString(t, bareInvoice(t))
	require.NoError(t, err)
	assert.Nil(t, inv.Totals.Rounding, "no rounding on the wire means none in GOBL")
}
