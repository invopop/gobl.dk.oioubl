package oioubl_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRejectsLineWithoutPrice(t *testing.T) {
	// The generic parser silently drops a line with no price, losing its amount
	// and shifting every later line's excise duties onto the wrong product.
	doc := strings.Replace(bareInvoice(t),
		`<cac:Price>
      <cbc:PriceAmount currencyID="DKK">31676.00</cbc:PriceAmount>
    </cac:Price>`, "", 1)

	_, err := convertString(t, doc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "line 1 has no price")
}
