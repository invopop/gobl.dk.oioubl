package oioubl_test

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invopop/gobl"
	oioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/invopop/gobl.dk.oioubl/addon"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
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

			doc, err := oioubl.ConvertInvoice(env)
			require.NoError(t, err)

			data, err := oioubl.Bytes(doc)
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

	out, err := oioubl.Convert(env)
	require.NoError(t, err)
	assert.NotNil(t, out)
}

// TestConvertUnsupportedVATKey checks that a VAT key OIOUBL cannot express is
// left without a taxcategoryid rather than defaulting to StandardRated: the
// document then fails F-LIB074 instead of going out as a mislabelled 0% supply.
func TestConvertUnsupportedVATKey(t *testing.T) {
	party := func(name, code string) *org.Party {
		return &org.Party{
			Name:      name,
			TaxID:     &tax.Identity{Country: "DK", Code: cbc.Code(code)},
			Addresses: []*org.Address{{Street: "Hovedgaden", Locality: "København", Code: "1000", Country: "DK"}},
		}
	}
	for _, key := range []cbc.Key{tax.KeyOutsideScope, tax.KeyIntraCommunity, tax.KeyExport} {
		t.Run(key.String(), func(t *testing.T) {
			inv := &bill.Invoice{
				Regime:    tax.WithRegime("DK"),
				Addons:    tax.WithAddons(en16931.V2017, addon.V2),
				IssueDate: cal.MakeDate(2026, 1, 1),
				Type:      "standard",
				Series:    "2026",
				Code:      "1",
				Currency:  "DKK",
				Supplier:  party("Eksempel A/S", "12345674"),
				Customer:  party("Kunde ApS", "88146328"),
				Lines: []*bill.Line{{
					Quantity: num.MakeAmount(1, 0),
					Item:     &org.Item{Name: "vare", Price: num.NewAmount(10000, 2)},
					Taxes:    tax.Set{{Category: "VAT", Key: key}},
				}},
			}
			env, err := gobl.Envelop(inv)
			require.NoError(t, err)
			out, err := oioubl.ConvertInvoice(env)
			require.NoError(t, err)
			data, err := xml.MarshalIndent(out, "", " ")
			require.NoError(t, err)
			assert.NotContains(t, string(data), "taxcategoryid-1.1",
				"an unsupported VAT key must not be given a taxcategoryid")
			assert.NotContains(t, string(data), "StandardRated",
				"and must never default to StandardRated")
		})
	}
}

// The Danish bank registration number rides pay.CreditTransfer.Clearing, which
// the addon requires for a domestic transfer, and OIOUBL wants in
// FinancialInstitutionBranch (F-LIB124/F-LIB130). The account name is the
// bank's own and has to survive the move.
func TestConvertDomesticTransferClearing(t *testing.T) {
	env := loadTestEnvelope(t, filepath.Join(getConvertPath(), "dk-bank.json"))
	require.NoError(t, env.Calculate())
	require.NoError(t, env.Validate(), "fixture must satisfy the addon's own rules")

	doc, err := oioubl.ConvertInvoice(env)
	require.NoError(t, err)

	require.NotEmpty(t, doc.PaymentMeans)
	account := doc.PaymentMeans[0].PayeeFinancialAccount
	require.NotNil(t, account)
	require.NotNil(t, account.FinancialInstitutionBranch)
	require.NotNil(t, account.FinancialInstitutionBranch.ID)
	assert.Equal(t, "1234", *account.FinancialInstitutionBranch.ID)
	require.NotNil(t, account.Name)
	assert.Equal(t, "Danske Bank", *account.Name)
}
