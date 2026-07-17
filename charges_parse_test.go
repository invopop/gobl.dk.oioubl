package dkoioubl

import (
	"testing"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/num"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A document-level 0% ZeroRated charge must keep an explicit 0% Percent, or
// GOBL will normalize it the same as exempt/reverse-charge instead of "zero".
func TestParseAllowanceChargeZeroRated(t *testing.T) {
	zero := "0"
	id := IDType{Value: "ZeroRated"}
	ac := &AllowanceCharge{
		ChargeIndicator: true,
		Amount:          Amount{Value: "10"},
		TaxCategory: []*TaxCategory{{
			ID:      &id,
			Percent: &zero,
			TaxScheme: &TaxScheme{
				ID: IDType{Value: "63"},
			},
		}},
	}

	p, err := parseAllowanceCharge(ac, untdid.ExtKeyCharge, nil)
	require.NoError(t, err)
	require.Len(t, p.Taxes, 1)
	require.NotNil(t, p.Taxes[0].Percent)
	assert.True(t, p.Taxes[0].Percent.IsZero())
}

func TestLineChargeMirrors(t *testing.T) {
	lines := []*bill.Line{
		{Charges: []*bill.LineCharge{
			{Reason: "Fragt", Amount: num.MakeAmount(1500, 2)},
			// An excise-keyed line charge is a different mirroring mechanism
			// (lineExciseMirrors) and must not pollute the ordinary-charge count.
			{Key: "excise", Reason: "Fragt", Amount: num.MakeAmount(1500, 2)},
		}},
		{Discounts: []*bill.LineDiscount{
			{Reason: "Rabat", Amount: num.MakeAmount(500, 2)},
		}},
	}

	fragt := num.MakeAmount(1500, 2)
	rabat := num.MakeAmount(500, 2)

	chargeMirrors := lineChargeMirrors(lines)
	assert.Equal(t, 1, chargeMirrors[chargeMirrorKey("Fragt", fragt, nil)])
	assert.Equal(t, 0, chargeMirrors[chargeMirrorKey("Rabat", rabat, nil)])

	discountMirrors := lineDiscountMirrors(lines)
	assert.Equal(t, 1, discountMirrors[chargeMirrorKey("Rabat", rabat, nil)])
	assert.Equal(t, 0, discountMirrors[chargeMirrorKey("Fragt", fragt, nil)])
}

func TestGoblAddChargesSkipsLineMirrorButKeepsGenuineDocumentCharge(t *testing.T) {
	// "Fragt" mirrors the line's own charge below (the OIOUBL-mandated
	// document-level rollup of that one line charge, F-INV128/F-INV130) and
	// must not become a second, separate bill.Charge. "Håndteringsgebyr" has
	// no line-level counterpart and must survive the dedup (the bug the
	// excise-side test guards against: dropping ALL document charges the
	// moment ANY line has its own, per TestExciseChargesFromTaxTotals).
	ui := &Invoice{
		AllowanceCharge: []AllowanceCharge{
			{
				ChargeIndicator:         true,
				AllowanceChargeReason:   strPtr("Fragt"),
				Amount:                  Amount{Value: "15.00"},
				MultiplierFactorNumeric: strPtr("0.1"),
				BaseAmount:              &Amount{Value: "150.00"},
			},
			{
				ChargeIndicator:       true,
				AllowanceChargeReason: strPtr("Håndteringsgebyr"),
				Amount:                Amount{Value: "20.00"},
			},
		},
	}
	fragtBase := num.MakeAmount(15000, 2)
	out := &bill.Invoice{
		Lines: []*bill.Line{
			{Charges: []*bill.LineCharge{
				// Base matches the document-level mirror's own BaseAmount
				// above (150.00): a genuine mirror restates the same
				// percent/base/amount, not just the reason and amount.
				{Reason: "Fragt", Amount: num.MakeAmount(1500, 2), Base: &fragtBase},
			}},
		},
	}

	require.NoError(t, ui.goblAddCharges(out))
	require.Len(t, out.Charges, 1)
	assert.Equal(t, "Håndteringsgebyr", out.Charges[0].Reason)
}

// TestGoblAddChargesKeepsChargeSharingReasonAndAmountButDifferentBase guards
// Gap A for ordinary (non-excise) charges: a document-level charge that
// merely coincides in reason and amount with a line's own -- but was computed
// against a different base -- is not actually a mirror and must survive.
func TestGoblAddChargesKeepsChargeSharingReasonAndAmountButDifferentBase(t *testing.T) {
	ui := &Invoice{
		AllowanceCharge: []AllowanceCharge{
			{
				ChargeIndicator:         true,
				AllowanceChargeReason:   strPtr("Fragt"),
				Amount:                  Amount{Value: "15.00"},
				MultiplierFactorNumeric: strPtr("0.1"),
				BaseAmount:              &Amount{Value: "150.00"},
			},
		},
	}
	lineBase := num.MakeAmount(20000, 2)
	out := &bill.Invoice{
		Lines: []*bill.Line{
			{Charges: []*bill.LineCharge{
				{Reason: "Fragt", Amount: num.MakeAmount(1500, 2), Base: &lineBase},
			}},
		},
	}

	require.NoError(t, ui.goblAddCharges(out))
	require.Len(t, out.Charges, 1, "same reason/amount but a different base means these are two distinct charges")
	assert.Equal(t, "Fragt", out.Charges[0].Reason)
}

func strPtr(s string) *string { return &s }
