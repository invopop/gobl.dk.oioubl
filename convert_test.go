package dkoioubl_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invopop/gobl"
	dkoioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/tax"
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

// A supplier whose country has no GOBL regime still yields a tax identity, so
// nothing upstream rejects the invoice and RegimeDef() is nil all the way into
// the totals builder. It has to convert rather than panic.
func TestConvertWithoutRegime(t *testing.T) {
	party := func(name string) *org.Party {
		return &org.Party{
			Name:       name,
			TaxID:      &tax.Identity{Country: "AF", Code: "12345674"},
			Endpoints:  []*org.Endpoint{{URI: "DK:CVR:12345674"}},
			Identities: []*org.Identity{{Scope: org.IdentityScopeLegal, Code: "12345674"}},
			Addresses:  []*org.Address{{Street: "Hovedgaden", Locality: "Kabul", Code: "1000", Country: "AF"}},
			People:     []*org.Person{{Name: &org.Name{Given: "Ali"}, Identities: []*org.Identity{{Code: "1"}}}},
		}
	}
	inv := &bill.Invoice{
		IssueDate: cal.MakeDate(2026, 1, 1),
		Code:      "1",
		Currency:  "DKK",
		Supplier:  party("Ukendt A/S"),
		Customer:  party("Kunde A/S"),
		Lines: []*bill.Line{{
			Quantity: num.MakeAmount(1, 0),
			Item:     &org.Item{Name: "vare", Price: num.NewAmount(10000, 2)},
			Taxes:    tax.Set{{Category: "VAT", Percent: num.NewPercentage(25, 2)}},
		}},
		Payment: &bill.PaymentDetails{
			Terms: &pay.Terms{Notes: "Net 30"},
			Instructions: &pay.Instructions{
				Key:            pay.MeansKeyDebitTransfer,
				CreditTransfer: []*pay.CreditTransfer{{IBAN: "DK5000400440116243", BIC: "DABADKKK"}},
			},
		},
	}
	env, err := gobl.Envelop(inv)
	require.NoError(t, err)
	require.Nil(t, inv.RegimeDef(), "fixture must have no regime for this test to mean anything")

	out, err := dkoioubl.Convert(env)
	require.NoError(t, err)
	assert.NotNil(t, out)
}
