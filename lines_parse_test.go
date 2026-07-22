package dkoioubl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cac:Price/cac:AllowanceCharge is advisory only (G17 3.3: already priced
// into PriceAmount) -- it must become a line note, not a charge, and must
// not be silently dropped either.
func TestGoblConvertLinePriceAllowanceCharge(t *testing.T) {
	docLine := &InvoiceLine{
		Price: &Price{
			PriceAmount: Amount{Value: "1000"},
			AllowanceCharge: &AllowanceCharge{
				ChargeIndicator:         true,
				AllowanceChargeReason:   strPtr("Emballageafgift"),
				MultiplierFactorNumeric: strPtr("0.05"),
				Amount:                  Amount{Value: "50"},
				BaseAmount:              &Amount{Value: "1000"},
			},
		},
	}

	line, err := goblConvertLine(docLine, nil)
	require.NoError(t, err)
	require.NotNil(t, line)
	assert.Empty(t, line.Charges)
	require.Len(t, line.Notes, 1)
	assert.Equal(t, "Emballageafgift", line.Notes[0].Text)
}

// A line-level cac:AllowanceCharge and a cac:Price/cac:AllowanceCharge can
// both be present at once; both are advisory (G17 3.2/3.3) and become notes, in order.
func TestGoblConvertLineCombinesOrdinaryAndPriceAllowanceCharges(t *testing.T) {
	docLine := &InvoiceLine{
		AllowanceCharge: []*AllowanceCharge{{
			ChargeIndicator:       true,
			AllowanceChargeReason: strPtr("Fragt"),
			Amount:                Amount{Value: "15"},
		}},
		Price: &Price{
			PriceAmount: Amount{Value: "1000"},
			AllowanceCharge: &AllowanceCharge{
				ChargeIndicator:       true,
				AllowanceChargeReason: strPtr("Emballageafgift"),
				Amount:                Amount{Value: "50"},
			},
		},
	}

	line, err := goblConvertLine(docLine, nil)
	require.NoError(t, err)
	require.NotNil(t, line)
	assert.Empty(t, line.Charges)
	require.Len(t, line.Notes, 2)
	assert.Equal(t, "Fragt", line.Notes[0].Text)
	assert.Equal(t, "Emballageafgift", line.Notes[1].Text)
}
