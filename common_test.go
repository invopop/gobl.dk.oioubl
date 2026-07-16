package dkoioubl

import (
	"testing"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
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

func TestGoblVATKey(t *testing.T) {
	assert.Equal(t, tax.KeyStandard, goblVATKey("StandardRated"))
	// ZeroRated maps to "zero" (not "exempt"), matching the line-parse path.
	assert.Equal(t, tax.KeyZero, goblVATKey("ZeroRated"))
	assert.Equal(t, tax.KeyReverseCharge, goblVATKey("ReverseCharge"))
	// Unknown values yield no key rather than guessing.
	assert.Equal(t, cbc.Key(""), goblVATKey("Excise"))
}
