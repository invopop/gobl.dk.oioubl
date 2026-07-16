package dkoioubl

import (
	"testing"

	oioubl "github.com/invopop/gobl.dk.oioubl/addon"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
)

func TestChargeDutyCode(t *testing.T) {
	// The excise key marks the duty; the SKAT code rides the extension.
	assert.True(t, chargeIsExcise(oioubl.ChargeKeyExcise))
	assert.False(t, chargeIsExcise(cbc.Key("stamp-duty")))
	assert.False(t, chargeIsExcise(cbc.Key("")))

	assert.Equal(t, "16", chargeDutyCode(dutyCodeExt("16")))
	assert.Equal(t, "21d", chargeDutyCode(dutyCodeExt("21d")))
	assert.Equal(t, "", chargeDutyCode(tax.Extensions{}))
}
