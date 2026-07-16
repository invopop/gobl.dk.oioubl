package dkoioubl

import (
	"testing"

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
