package dkoioubl_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dkoioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getParsePath() string {
	return filepath.Join("test", "data", "parse")
}

// Fixtures that genuinely fail Convert() for reasons unrelated to a bug here
// (bad sample data, a gobl.ubl gap); see PR description.
var knownInvalid = map[string]string{
	"negative-rounding_real.xml": "GOBL-TAX-COMBO-04",
	"used-invoice_real.xml":      "GOBL-CBC-KEY-02",
}

// normalizeJSONField overwrites a top-level JSON field with a fixed
// placeholder, so a non-reproducible value doesn't break a golden diff.
func normalizeJSONField(t *testing.T, data []byte, field string) []byte {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	if _, ok := m[field]; ok {
		m[field] = "00000000-0000-0000-0000-000000000000"
	}
	out, err := json.MarshalIndent(m, "", "\t")
	require.NoError(t, err)
	return out
}

func TestParseInvoice(t *testing.T) {
	examples, err := filepath.Glob(filepath.Join(getParsePath(), "*.xml"))
	require.NoError(t, err)
	require.NotEmpty(t, examples, "no OIOUBL examples found")

	for _, example := range examples {
		inName := filepath.Base(example)
		outName := strings.Replace(inName, ".xml", ".json", 1)

		t.Run(inName, func(t *testing.T) {
			data, err := os.ReadFile(example)
			require.NoError(t, err)

			doc, err := dkoioubl.ParseInvoice(data)
			require.NoError(t, err)

			env, err := doc.Convert()
			if wantErr, ok := knownInvalid[inName]; ok {
				assert.ErrorContains(t, err, wantErr)
				return
			}
			require.NoError(t, err)

			// Compare the invoice content only: the envelope wrapper (head.uuid,
			// its digest) is freshly minted on every parse and never reproducible.
			out, err := json.MarshalIndent(env.Extract(), "", "\t")
			require.NoError(t, err)
			// No wire UUID means a fresh one gets minted every parse; normalize it away.
			if doc.UUID == "" {
				out = normalizeJSONField(t, out, "uuid")
			}
			out = append(out, '\n')

			outPath := filepath.Join(getParsePath(), "out", outName)
			if *update {
				require.NoError(t, os.WriteFile(outPath, out, 0644))
			}

			expected, err := os.ReadFile(outPath)
			assert.NoError(t, err)
			assert.JSONEq(t, string(expected), string(out), "Output should match the expected GOBL JSON. Update with --update flag.")
		})
	}
}
