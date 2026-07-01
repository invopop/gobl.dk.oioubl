package addon

import (
	"testing"

	"github.com/invopop/gobl/cbc"
	"github.com/stretchr/testify/assert"
)

func TestOioublTaxCategory(t *testing.T) {
	assert.Equal(t, ExtValueTaxCategoryStandardRated, oioublTaxCategory("S"))
	assert.Equal(t, ExtValueTaxCategoryZeroRated, oioublTaxCategory("Z"))
	assert.Equal(t, ExtValueTaxCategoryZeroRated, oioublTaxCategory("E"), "exempt reports as ZeroRated")
	assert.Equal(t, ExtValueTaxCategoryReverseCharge, oioublTaxCategory("AE"))
	assert.Equal(t, cbc.Code(""), oioublTaxCategory("X"))
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
