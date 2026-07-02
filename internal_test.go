package dkoioubl

import (
	"testing"

	"github.com/invopop/gobl/cbc"
	"github.com/stretchr/testify/assert"
)

func TestGoblTaxSchemeCategory(t *testing.T) {
	// OIOUBL's VAT scheme code maps back to the GOBL VAT category.
	assert.Equal(t, cbc.Code("VAT"), goblTaxSchemeCategory("63"))
	// A value already carrying the GOBL code passes through unchanged.
	assert.Equal(t, cbc.Code("VAT"), goblTaxSchemeCategory("VAT"))
	assert.Equal(t, cbc.Code("OSS"), goblTaxSchemeCategory("OSS"))
}

func TestGoblTaxCategoryCode(t *testing.T) {
	// OIOUBL category names map back to the UNTDID 5305 codes.
	assert.Equal(t, cbc.Code("S"), goblTaxCategoryCode("StandardRated"))
	assert.Equal(t, cbc.Code("Z"), goblTaxCategoryCode("ZeroRated"))
	assert.Equal(t, cbc.Code("AE"), goblTaxCategoryCode("ReverseCharge"))
	// Already-UNTDID values pass through unchanged.
	assert.Equal(t, cbc.Code("S"), goblTaxCategoryCode("S"))
	assert.Equal(t, cbc.Code("E"), goblTaxCategoryCode("E"))
}

func TestKortart(t *testing.T) {
	// FIK (93): no ref -> 73, 16 chars -> 75, anything else -> 71.
	assert.Equal(t, "73", kortart("93", ""))
	assert.Equal(t, "75", kortart("93", "1234567890123456"))
	assert.Equal(t, "71", kortart("93", "123456789012345"))
	// Giro (50): no ref -> 01, ref -> 04.
	assert.Equal(t, "01", kortart("50", ""))
	assert.Equal(t, "04", kortart("50", "12345678"))
	// Other means carry no kortart.
	assert.Equal(t, "", kortart("31", "x"))
}

func TestChargeExciseScheme(t *testing.T) {
	// An all-digit charge key marks an excise duty; the zero pad is stripped.
	assert.Equal(t, "16", chargeExciseScheme(cbc.Key("16")))
	assert.Equal(t, "9", chargeExciseScheme(cbc.Key("09")))
	assert.Equal(t, "0", chargeExciseScheme(cbc.Key("00")))
	assert.Equal(t, "", chargeExciseScheme(cbc.Key("stamp-duty")))
	assert.Equal(t, "", chargeExciseScheme(cbc.Key("")))
	// The inverse zero-pads a single digit so it is a valid cbc.Key.
	assert.Equal(t, cbc.Key("09"), exciseSchemeKey("9"))
	assert.Equal(t, cbc.Key("16"), exciseSchemeKey("16"))
}
