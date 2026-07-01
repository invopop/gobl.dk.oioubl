package addon

import (
	"testing"

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
