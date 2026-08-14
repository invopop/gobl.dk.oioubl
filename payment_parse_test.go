package oioubl_test

import (
	"strings"
	"testing"

	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePaymentMeansWithoutChannelCode(t *testing.T) {
	// The channel is optional and OIOUBL's own samples omit it, so the wire
	// means code still has to survive.
	doc := bareInvoice(t)
	const channel = `<cbc:PaymentChannelCode listID="urn:oioubl:codelist:paymentchannelcode-1.1">IBAN</cbc:PaymentChannelCode>`
	require.Contains(t, doc, channel)
	doc = strings.Replace(doc, channel, "", 1)

	inv, err := convertString(t, doc)
	require.NoError(t, err)
	require.NotNil(t, inv.Payment)
	require.NotNil(t, inv.Payment.Instructions)
	assert.Equal(t, "31", inv.Payment.Instructions.Ext.Get(untdid.ExtKeyPaymentMeans).String(),
		"the wire means code must not be replaced by the one GOBL re-derives")
}

func TestParsePaymentMeansWithChannelCode(t *testing.T) {
	inv, err := convertString(t, bareInvoice(t))
	require.NoError(t, err)
	require.NotNil(t, inv.Payment)
	require.NotNil(t, inv.Payment.Instructions)
	assert.Equal(t, "31", inv.Payment.Instructions.Ext.Get(untdid.ExtKeyPaymentMeans).String())
	// IBAN carries the BIC a level deeper than the base parser looks.
	require.Len(t, inv.Payment.Instructions.CreditTransfer, 1)
	assert.Equal(t, "DABADKKK", inv.Payment.Instructions.CreditTransfer[0].BIC.String())
}
