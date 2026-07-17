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
	mirrorBase := num.MakeAmount(1000, 2)
	mirrors := lineExciseMirrors([]*bill.Line{
		{Charges: []*bill.LineCharge{
			// Base matches the mirrored document-level TaxTotal's own
			// TaxableAmount below: a genuine mirror restates the same duty,
			// base included, not just the same code and amount (see Gap A:
			// an unrelated document-level duty could otherwise share the
			// same code/amount on a different base and be wrongly dropped).
			{Key: oioubl.ChargeKeyExcise, Ext: dutyCodeExt("16"), Amount: num.MakeAmount(1000, 2), Base: &mirrorBase},
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

// TestExciseChargesFromTaxTotals_SameCodeAndAmountDifferentBase guards Gap A:
// a line-level duty and an unrelated document-level duty can coincidentally
// share the same duty code and amount while being computed against different
// bases. Keying the mirror dedup on code+amount alone would wrongly treat the
// document-level one as the line's mirror and silently drop it.
func TestExciseChargesFromTaxTotals_SameCodeAndAmountDifferentBase(t *testing.T) {
	lineBase := num.MakeAmount(4000, 2) // 40.00 @ 25%
	mirrors := lineExciseMirrors([]*bill.Line{
		{Charges: []*bill.LineCharge{
			{Key: oioubl.ChargeKeyExcise, Ext: dutyCodeExt("16"), Amount: num.MakeAmount(1000, 2), Base: &lineBase},
		}},
	})

	// A genuinely different, document-only duty: same code "16" and amount
	// 10.00 as the line's, but computed on its own 50.00 base (@ 20%).
	docOnly := TaxTotal{
		TaxAmount: Amount{Value: "10.00"},
		TaxSubtotal: []TaxSubtotal{{
			TaxableAmount: Amount{Value: "50.00"},
			TaxAmount:     Amount{Value: "10.00"},
			TaxCategory: TaxCategory{
				ID:        &IDType{Value: taxCategoryExcise},
				TaxScheme: &TaxScheme{ID: IDType{Value: "16"}},
			},
		}},
	}

	charges, err := exciseChargesFromTaxTotals([]TaxTotal{docOnly}, mirrors)
	require.NoError(t, err)
	require.Len(t, charges, 1, "the document-level duty must survive: it is not actually a mirror of the line's")
	assert.Equal(t, "16", chargeDutyCode(charges[0].Ext))
	require.NotNil(t, charges[0].Base)
	assert.Equal(t, "50.00", charges[0].Base.String())
}
