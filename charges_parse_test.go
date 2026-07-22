package dkoioubl

import (
	"testing"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
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

// Per OIOUBL's guideline (G17 3.5), every header-level AllowanceCharge is
// real money regardless of what a line happens to carry -- there's no mirror
// to dedup against any more, since line-level entries never become bill.Charge.
func TestGoblAddChargesAlwaysKeepsEveryHeaderEntry(t *testing.T) {
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
	out := &bill.Invoice{}

	require.NoError(t, ui.goblAddCharges(out))
	require.Len(t, out.Charges, 2)
	assert.Equal(t, "Fragt", out.Charges[0].Reason)
	assert.Equal(t, "Håndteringsgebyr", out.Charges[1].Reason)
}

func strPtr(s string) *string { return &s }
