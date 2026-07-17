package dkoioubl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cac:Price/cac:AllowanceCharge (a price-level adjustment, e.g. a packaging
// fee baked into how the unit price was derived) must map onto the line the
// same way an ordinary cac:InvoiceLine/cac:AllowanceCharge does, not be
// silently dropped.
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
	require.Len(t, line.Charges, 1)
	assert.Equal(t, "Emballageafgift", line.Charges[0].Reason)
	assert.Equal(t, "50", line.Charges[0].Amount.String())
	require.NotNil(t, line.Charges[0].Base)
	assert.Equal(t, "1000", line.Charges[0].Base.String())
}

// A line-level cac:AllowanceCharge and a cac:Price/cac:AllowanceCharge can
// both be present at once; both must be captured, in order.
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
	require.Len(t, line.Charges, 2)
	assert.Equal(t, "Fragt", line.Charges[0].Reason)
	assert.Equal(t, "Emballageafgift", line.Charges[1].Reason)
}
