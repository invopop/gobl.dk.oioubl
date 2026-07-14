package dkoioubl

import (
	"testing"

	"github.com/invopop/gobl/cbc"
	"github.com/stretchr/testify/assert"
)

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
