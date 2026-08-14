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

func TestConvertAcceptsPrepaidPayable(t *testing.T) {
	// A prepaid document states its payable net of the prepayment; GOBL keeps
	// that as Due, and the stated-payable check has to compare against it.
	doc := bareInvoice(t)
	doc = strings.Replace(doc,
		`<cbc:PayableAmount currencyID="DKK">41178.80</cbc:PayableAmount>`,
		`<cbc:PrepaidAmount currencyID="DKK">1000.00</cbc:PrepaidAmount>
    <cbc:PayableAmount currencyID="DKK">40178.80</cbc:PayableAmount>`, 1)

	inv, err := convertString(t, doc)
	require.NoError(t, err)
	require.NotNil(t, inv.Totals.Due)
	assert.Equal(t, "40178.80", inv.Totals.Due.String())
}

func TestConvertRejectsMismatchedPayable(t *testing.T) {
	// The converter must not change what the customer owes: a document whose
	// stated payable does not match the converted total is refused, not fixed.
	doc := strings.Replace(bareInvoice(t),
		`<cbc:PayableAmount currencyID="DKK">41178.80</cbc:PayableAmount>`,
		`<cbc:PayableAmount currencyID="DKK">99999.99</cbc:PayableAmount>`, 1)

	_, err := convertString(t, doc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stated payable")
	assert.Contains(t, err.Error(), "99999.99")
}

func TestParseWithoutPayableRounding(t *testing.T) {
	inv, err := convertString(t, bareInvoice(t))
	require.NoError(t, err)
	assert.Nil(t, inv.Totals.Rounding, "no rounding on the wire means none in GOBL")
}
