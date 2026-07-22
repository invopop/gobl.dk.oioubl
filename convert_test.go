package dkoioubl_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invopop/gobl"
	dkoioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const jsonPattern = "*.json"

func getConvertPath() string {
	return filepath.Join("test", "data", "convert")
}

// loadTestEnvelope loads a GOBL envelope from a JSON file path.
func loadTestEnvelope(t *testing.T, path string) *gobl.Envelope {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	env := new(gobl.Envelope)
	require.NoError(t, json.Unmarshal(data, env))
	return env
}

func TestConvertInvoice(t *testing.T) {
	examples, err := filepath.Glob(filepath.Join(getConvertPath(), jsonPattern))
	require.NoError(t, err)
	require.NotEmpty(t, examples, "no invoice examples found")

	for _, example := range examples {
		inName := filepath.Base(example)
		outName := strings.Replace(inName, ".json", ".xml", 1)

		t.Run(inName, func(t *testing.T) {
			env := loadTestEnvelope(t, example)

			doc, err := dkoioubl.ConvertInvoice(env)
			require.NoError(t, err)

			data, err := dkoioubl.Bytes(doc)
			require.NoError(t, err)

			outPath := filepath.Join(getConvertPath(), "out", outName)
			if *update {
				require.NoError(t, os.WriteFile(outPath, data, 0644))
			}

			output, err := os.ReadFile(outPath)
			assert.NoError(t, err)
			assert.Equal(t, string(output), string(data), "Output should match the expected XML. Update with --update flag.")
		})
	}
}
