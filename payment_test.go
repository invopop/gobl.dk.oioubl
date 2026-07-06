package dkoioubl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKortart(t *testing.T) {
	// FIK (93): no ref -> 73, 16 chars -> 75, anything else -> 71.
	assert.Equal(t, "73", kortart("93", ""))
	assert.Equal(t, "75", kortart("93", "1234567890123456"))
	assert.Equal(t, "71", kortart("93", "123456789012345"))
	// Giro (50): no ref -> 01, ref -> 04.
	assert.Equal(t, "01", kortart("50", ""))
	assert.Equal(t, "04", kortart("50", "12345678"))
	// Other means carry no kortart.
	assert.Equal(t, "", kortart("31", "x"))
}
