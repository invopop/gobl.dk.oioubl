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

func TestIsExciseCategoryID(t *testing.T) {
	// The current codelist value.
	assert.True(t, isExciseCategoryID("Excise"))

	// The legacy UNCL5305 numeric block (3010-3671): lossily treated as excise
	// on parse rather than silently dropping a real duty.
	assert.True(t, isExciseCategoryID("3010"))
	assert.True(t, isExciseCategoryID("3030"))
	assert.True(t, isExciseCategoryID("3671"))

	// Out of range or not a legacy/current excise value at all.
	assert.False(t, isExciseCategoryID("3009"))
	assert.False(t, isExciseCategoryID("3672"))
	assert.False(t, isExciseCategoryID("StandardRated"))
	assert.False(t, isExciseCategoryID(""))
}
