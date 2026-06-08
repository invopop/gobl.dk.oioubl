package oioubl_test

import (
	"testing"

	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the EN 16931 over-enforcement relaxations that live in
// gobl core (addons/eu/en16931) but only apply when the dk-oioubl addon is
// present. They moved here from the en16931 test suite when the addon was
// externalised: core can keep the production carve-outs (they gate on the
// literal addon key) but must not test-depend on this module.

func TestEN16931CarveOuts(t *testing.T) {
	t.Run("due invoice without payment skips BR-CO-25", func(t *testing.T) {
		// OIOUBL's payment rules are all conditional-on-presence, so a due
		// invoice with no payment means/terms must not trip EN 16931's BR-CO-25.
		inv := testInvoiceStandard(t)
		inv.Payment = nil
		require.NoError(t, inv.Calculate())
		if err := rules.Validate(inv); err != nil {
			assert.NotContains(t, err.Error(), "payment details are required")
			assert.NotContains(t, err.Error(), "payment terms are required")
		}
	})

	t.Run("exempt without reason skips the exemption rules (BR-E-10)", func(t *testing.T) {
		// OIOUBL has no exempt tax category (exempt maps to ZeroRated), so the
		// EN 16931 rules requiring an exemption reason must not fire.
		inv := testInvoiceStandard(t)
		inv.Lines[0].Taxes = tax.Set{{Category: tax.CategoryVAT, Key: tax.KeyExempt}}
		require.NoError(t, inv.Calculate())
		if err := rules.Validate(inv); err != nil {
			assert.NotContains(t, err.Error(), "exempt tax categories require")
		}
	})
}
