package dkoioubl_test

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invopop/gobl"
	dkoioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/uuid"
	"github.com/invopop/phive"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	xmlPattern  = "*.xml"
	jsonPattern = "*.json"

	staticUUID uuid.UUID = "0195ce71-dc9c-72c8-bf2c-9890a4a9f0a2"
)

// validate enables schematron validation of the generated XML against a local
// phive service on 127.0.0.1:9090.
var validate = flag.Bool("validate", false, "Run phive validation on generated XML")

// phiveClient connects to the local phive service when -validate is set,
// returning nil otherwise.
func phiveClient(t *testing.T) phive.ValidationServiceClient {
	t.Helper()
	if !*validate {
		return nil
	}
	conn, err := grpc.NewClient(
		"127.0.0.1:9090",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return phive.NewValidationServiceClient(conn)
}

// validateXML runs the generated document through phive against the given VESID.
func validateXML(t *testing.T, pc phive.ValidationServiceClient, vesid string, data []byte) {
	t.Helper()
	if pc == nil {
		return
	}
	resp, err := pc.ValidateXml(context.Background(), &phive.ValidateXmlRequest{
		Vesid:      vesid,
		XmlContent: data,
	})
	require.NoError(t, err)
	results, err := json.MarshalIndent(resp.Results, "", "  ")
	require.NoError(t, err)
	require.True(t, resp.Success, "Generated XML should be valid for %s: %s", vesid, string(results))
}

func getConvertPath() string {
	return filepath.Join("test", "data", "convert")
}

func getParsePath() string {
	return filepath.Join("test", "data", "parse")
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
	pc := phiveClient(t)

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

			if inv, ok := env.Extract().(*bill.Invoice); ok {
				validateXML(t, pc, dkoioubl.GetVESID(inv), data)
			}

			output, err := os.ReadFile(outPath)
			assert.NoError(t, err)
			assert.Equal(t, string(output), string(data), "Output should match the expected XML. Update with --update flag.")
		})
	}
}

func TestParseInvoice(t *testing.T) {
	examples, err := filepath.Glob(filepath.Join(getParsePath(), xmlPattern))
	require.NoError(t, err)
	require.NotEmpty(t, examples, "no invoice parse examples found")

	for _, example := range examples {
		inName := filepath.Base(example)
		outName := strings.Replace(inName, ".xml", ".json", 1)

		t.Run(inName, func(t *testing.T) {
			xmlData, err := os.ReadFile(example)
			require.NoError(t, err)

			doc, err := dkoioubl.Parse(xmlData)
			require.NoError(t, err)
			inv, ok := doc.(*dkoioubl.Invoice)
			require.True(t, ok, "Document should be an invoice")

			env, err := inv.Convert()
			require.NoError(t, err)

			env.Head.UUID = staticUUID
			if inv, ok := env.Extract().(*bill.Invoice); ok {
				inv.UUID = staticUUID
			}
			require.NoError(t, env.Calculate())

			outPath := filepath.Join(getParsePath(), "out", outName)
			if *update {
				data, err := json.MarshalIndent(env, "", "\t")
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(outPath, data, 0644))
			}

			invoice, ok := env.Extract().(*bill.Invoice)
			require.True(t, ok, "Document should be an invoice")
			data, err := json.MarshalIndent(invoice, "", "\t")
			require.NoError(t, err)

			output, err := os.ReadFile(outPath)
			assert.NoError(t, err)

			var expectedEnv gobl.Envelope
			require.NoError(t, json.Unmarshal(output, &expectedEnv))
			expectedInvoice, ok := expectedEnv.Extract().(*bill.Invoice)
			require.True(t, ok, "Expected document should be an invoice")
			expectedData, err := json.MarshalIndent(expectedInvoice, "", "\t")
			require.NoError(t, err)

			assert.JSONEq(t, string(expectedData), string(data), "Invoice should match the expected JSON. Update with --update flag.")
		})
	}
}

func TestConvertReminder(t *testing.T) {
	pc := phiveClient(t)

	examples, err := filepath.Glob(filepath.Join(getConvertPath(), "reminder", jsonPattern))
	require.NoError(t, err)
	require.NotEmpty(t, examples, "no Reminder examples found")

	for _, example := range examples {
		inName := filepath.Base(example)
		outName := strings.Replace(inName, ".json", ".xml", 1)

		t.Run(inName, func(t *testing.T) {
			env := loadTestEnvelope(t, example)

			doc, err := dkoioubl.Convert(env)
			require.NoError(t, err)

			data, err := dkoioubl.Bytes(doc)
			require.NoError(t, err)

			outPath := filepath.Join(getConvertPath(), "reminder", "out", outName)
			if *update {
				require.NoError(t, os.WriteFile(outPath, data, 0644))
			}

			validateXML(t, pc, dkoioubl.VESIDReminder, data)

			output, err := os.ReadFile(outPath)
			assert.NoError(t, err)
			assert.Equal(t, string(output), string(data), "Output should match the expected XML. Update with --update flag.")
		})
	}
}

func TestParseReminder(t *testing.T) {
	examples, err := filepath.Glob(filepath.Join(getParsePath(), "reminder", xmlPattern))
	require.NoError(t, err)
	require.NotEmpty(t, examples, "no Reminder parse examples found")

	for _, example := range examples {
		inName := filepath.Base(example)
		outName := strings.Replace(inName, ".xml", ".json", 1)

		t.Run(inName, func(t *testing.T) {
			xmlData, err := os.ReadFile(example)
			require.NoError(t, err)

			doc, err := dkoioubl.Parse(xmlData)
			require.NoError(t, err)
			rem, ok := doc.(*dkoioubl.Reminder)
			require.True(t, ok, "Document should be a Reminder")

			env, err := rem.Convert()
			require.NoError(t, err)

			env.Head.UUID = staticUUID
			if pmt, ok := env.Extract().(*bill.Payment); ok {
				pmt.UUID = staticUUID
			}
			require.NoError(t, env.Calculate())

			outPath := filepath.Join(getParsePath(), "reminder", "out", outName)
			if *update {
				data, err := json.MarshalIndent(env, "", "\t")
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(outPath, data, 0644))
			}

			payment, ok := env.Extract().(*bill.Payment)
			require.True(t, ok, "Document should be a payment")
			data, err := json.MarshalIndent(payment, "", "\t")
			require.NoError(t, err)

			output, err := os.ReadFile(outPath)
			assert.NoError(t, err)

			var expectedEnv gobl.Envelope
			require.NoError(t, json.Unmarshal(output, &expectedEnv))
			expectedPayment, ok := expectedEnv.Extract().(*bill.Payment)
			require.True(t, ok, "Expected document should be a payment")
			expectedData, err := json.MarshalIndent(expectedPayment, "", "\t")
			require.NoError(t, err)

			assert.JSONEq(t, string(expectedData), string(data), "Payment should match the expected JSON. Update with --update flag.")
		})
	}
}

func TestConvertResponse(t *testing.T) {
	pc := phiveClient(t)

	examples, err := filepath.Glob(filepath.Join(getConvertPath(), "response", jsonPattern))
	require.NoError(t, err)
	require.NotEmpty(t, examples, "no ApplicationResponse examples found")

	for _, example := range examples {
		inName := filepath.Base(example)
		outName := strings.Replace(inName, ".json", ".xml", 1)

		t.Run(inName, func(t *testing.T) {
			env := loadTestEnvelope(t, example)

			doc, err := dkoioubl.Convert(env)
			require.NoError(t, err)

			data, err := dkoioubl.Bytes(doc)
			require.NoError(t, err)

			outPath := filepath.Join(getConvertPath(), "response", "out", outName)
			if *update {
				require.NoError(t, os.WriteFile(outPath, data, 0644))
			}

			validateXML(t, pc, dkoioubl.VESIDApplicationResponse, data)

			output, err := os.ReadFile(outPath)
			assert.NoError(t, err)
			assert.Equal(t, string(output), string(data), "Output should match the expected XML. Update with --update flag.")
		})
	}
}

func TestParseResponse(t *testing.T) {
	examples, err := filepath.Glob(filepath.Join(getParsePath(), "response", xmlPattern))
	require.NoError(t, err)
	require.NotEmpty(t, examples, "no ApplicationResponse parse examples found")

	for _, example := range examples {
		inName := filepath.Base(example)
		outName := strings.Replace(inName, ".xml", ".json", 1)

		t.Run(inName, func(t *testing.T) {
			xmlData, err := os.ReadFile(example)
			require.NoError(t, err)

			doc, err := dkoioubl.Parse(xmlData)
			require.NoError(t, err)
			ar, ok := doc.(*dkoioubl.ApplicationResponse)
			require.True(t, ok, "Document should be an ApplicationResponse")

			env, err := ar.Convert()
			require.NoError(t, err)

			env.Head.UUID = staticUUID
			if st, ok := env.Extract().(*bill.Status); ok {
				st.UUID = staticUUID
			}
			require.NoError(t, env.Calculate())

			outPath := filepath.Join(getParsePath(), "response", "out", outName)
			if *update {
				data, err := json.MarshalIndent(env, "", "\t")
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(outPath, data, 0644))
			}

			status, ok := env.Extract().(*bill.Status)
			require.True(t, ok, "Document should be a status")
			data, err := json.MarshalIndent(status, "", "\t")
			require.NoError(t, err)

			output, err := os.ReadFile(outPath)
			assert.NoError(t, err)

			var expectedEnv gobl.Envelope
			require.NoError(t, json.Unmarshal(output, &expectedEnv))
			expectedStatus, ok := expectedEnv.Extract().(*bill.Status)
			require.True(t, ok, "Expected document should be a status")
			expectedData, err := json.MarshalIndent(expectedStatus, "", "\t")
			require.NoError(t, err)

			assert.JSONEq(t, string(expectedData), string(data), "Status should match the expected JSON. Update with --update flag.")
		})
	}
}
