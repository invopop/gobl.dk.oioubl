package addon

import (
	"testing"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func TestTaxCategoryMapsToOIOUBL(t *testing.T) {
	assert.True(t, taxCategoryMapsToOIOUBL(tax.KeyStandard))
	assert.True(t, taxCategoryMapsToOIOUBL(tax.KeyZero))
	assert.True(t, taxCategoryMapsToOIOUBL(tax.KeyExempt), "exempt reports as ZeroRated")
	assert.True(t, taxCategoryMapsToOIOUBL(tax.KeyReverseCharge))
	assert.False(t, taxCategoryMapsToOIOUBL(tax.KeyIntraCommunity))
}

func TestOioublPaymentChannel(t *testing.T) {
	assert.Equal(t, ExtValuePaymentChannelGiro, oioublPaymentChannel("50"))
	assert.Equal(t, ExtValuePaymentChannelFIK, oioublPaymentChannel("93"))
	assert.Equal(t, ExtValuePaymentChannelIBAN, oioublPaymentChannel("30"), "credit transfer settles to an account")
	assert.Equal(t, ExtValuePaymentChannelIBAN, oioublPaymentChannel("31"))
	assert.Equal(t, cbc.Code(""), oioublPaymentChannel("42"), "42 dropped: needs DK:BANK + branch-number modelling")
	assert.Equal(t, ExtValuePaymentChannelIBAN, oioublPaymentChannel("58"), "SEPA credit transfer settles to an account")
	assert.Equal(t, cbc.Code(""), oioublPaymentChannel("49"), "direct debit carries no channel")
	assert.Equal(t, cbc.Code(""), oioublPaymentChannel("10"), "cash carries no channel")
	assert.Equal(t, cbc.Code(""), oioublPaymentChannel("20"), "cheque carries no channel")
	assert.Equal(t, cbc.Code(""), oioublPaymentChannel("48"), "card payments carry no channel")
	assert.Equal(t, cbc.Code(""), oioublPaymentChannel(""))
}
