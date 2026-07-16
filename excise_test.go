package dkoioubl

import (
	"testing"

	oioubl "github.com/invopop/gobl.dk.oioubl/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func testExciseTotal(scheme, amount string) TaxTotal {
	return TaxTotal{
		TaxAmount: Amount{Value: amount},
		TaxSubtotal: []TaxSubtotal{{
			TaxableAmount: Amount{Value: amount},
			TaxAmount:     Amount{Value: amount},
			TaxCategory: TaxCategory{
				ID:        &IDType{Value: taxCategoryExcise},
				TaxScheme: &TaxScheme{ID: IDType{Value: scheme}},
			},
		}},
	}
}

func TestExciseChargesFromTaxTotals(t *testing.T) {
	// A document-level TaxTotal that matches a line's already-parsed excise
	// charge is a mirror and must be skipped, not double-counted (the bug
	// this test guards: goblAddExciseCharges used to skip ALL document-level
	// excise the moment ANY line had its own, dropping genuine document-only
	// duties like a car registration tax alongside a line-level one).
	mirrors := lineExciseMirrors([]*bill.Line{
		{Charges: []*bill.LineCharge{
			{Key: oioubl.ChargeKeyExcise, Ext: dutyCodeExt("16"), Amount: num.MakeAmount(1000, 2)},
		}},
	})

	totals := []TaxTotal{
		testExciseTotal("16", "10.00"),  // mirrors the line's own duty
		testExciseTotal("66", "1000.00"), // genuine document-only duty
	}

	charges, err := exciseChargesFromTaxTotals(totals, mirrors)
	require.NoError(t, err)
	require.Len(t, charges, 1)
	assert.Equal(t, "66", chargeDutyCode(charges[0].Ext))
}
