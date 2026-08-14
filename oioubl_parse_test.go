package oioubl_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	oioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertTwiceIsStable guards against stripping the caller's own document:
// a second Convert used to see a half-stripped one and silently drop its
// excise charges.
func TestConvertTwiceIsStable(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(getParsePath(), "excise-registration-tax_real.xml"))
	require.NoError(t, err)

	in, err := oioubl.ParseInvoice(data)
	require.NoError(t, err)

	first, err := in.Convert()
	require.NoError(t, err)
	second, err := in.Convert()
	require.NoError(t, err)

	// A document with no cbc:UUID gets a fresh one minted on every parse.
	assert.JSONEq(t, withoutUUID(t, first.Extract()), withoutUUID(t, second.Extract()),
		"converting the same document twice must give the same invoice")
}

func withoutUUID(t *testing.T, doc any) string {
	t.Helper()
	data, err := json.Marshal(doc)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	delete(m, "uuid")
	out, err := json.Marshal(m)
	require.NoError(t, err)
	return string(out)
}
