package dkoioubl_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invopop/gobl"
	dkoioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseXMLInvoice parses an XML fixture from test/data/parse into a GOBL envelope.
func parseXMLInvoice(t *testing.T, name string) *gobl.Envelope {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(getParsePath(), name))
	require.NoError(t, err)
	doc, err := dkoioubl.Parse(data)
	require.NoError(t, err)
	inv, ok := doc.(*dkoioubl.Invoice)
	require.True(t, ok)
	env, err := inv.Convert()
	require.NoError(t, err)
	return env
}

func TestPaymentMeans(t *testing.T) {
	t.Run("applies OIO payment mapping", func(t *testing.T) {
		doc := convertInvoiceFrom(t, "invoice-minimal.json")

		require.NotEmpty(t, doc.PaymentMeans)
		pm := doc.PaymentMeans[0]
		assert.Equal(t, "31", pm.PaymentMeansCode.Value)
		require.NotNil(t, pm.PaymentChannelCode)
		assert.Equal(t, "IBAN", pm.PaymentChannelCode.Value)
		require.NotNil(t, pm.PayeeFinancialAccount)
		require.NotNil(t, pm.PayeeFinancialAccount.ID)
		assert.Equal(t, "NO9386011117947", *pm.PayeeFinancialAccount.ID)
		require.NotNil(t, pm.PayeeFinancialAccount.FinancialInstitutionBranch)
		assert.Nil(t, pm.PayeeFinancialAccount.FinancialInstitutionBranch.ID)
		require.NotNil(t, pm.PayeeFinancialAccount.FinancialInstitutionBranch.FinancialInstitution)
		require.NotNil(t, pm.PayeeFinancialAccount.FinancialInstitutionBranch.FinancialInstitution.ID)
		assert.Equal(t, "DNBANOKK", *pm.PayeeFinancialAccount.FinancialInstitutionBranch.FinancialInstitution.ID)
	})

	t.Run("keeps explicit payment-channel", func(t *testing.T) {
		env := loadTestEnvelope(t, filepath.Join(getConvertPath(), "invoice-minimal.json"))

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		inv.Payment.Instructions.Meta = cbc.Meta{
			cbc.Key("payment-channel"): "ZZZ",
		}

		doc, err := dkoioubl.ConvertInvoice(env)
		require.NoError(t, err)
		require.NotEmpty(t, doc.PaymentMeans)
		require.NotNil(t, doc.PaymentMeans[0].PaymentChannelCode)
		assert.Equal(t, "ZZZ", doc.PaymentMeans[0].PaymentChannelCode.Value)
		assert.Equal(t, "31", doc.PaymentMeans[0].PaymentMeansCode.Value)
	})

	t.Run("rejects a due date without a date", func(t *testing.T) {
		env := loadTestEnvelope(t, filepath.Join(getConvertPath(), "invoice-bare.json"))

		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.NotNil(t, inv.Payment)
		require.NotNil(t, inv.Payment.Terms)
		require.Len(t, inv.Payment.Terms.DueDates, 1)

		// An incomplete due date is rejected during conversion: Convert validates
		// after auto-adding the addon, so the fault surfaces rather than the
		// converter dropping the date silently (or panicking).
		inv.Payment.Terms.DueDates[0].Date = nil
		_, err := dkoioubl.ConvertInvoice(env)
		require.Error(t, err)
	})
}

func TestParseDueDateAndNestedBIC(t *testing.T) {
	// OIOUBL moves the invoice due date onto the payment means (clearing the
	// root) and nests the BIC under FinancialInstitution/ID after stripping the
	// branch ID (F-LIB295). Both must survive the parse.
	env := parseXMLInvoice(t, "invoice-bare.xml")
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)
	require.NotNil(t, inv.Payment)

	require.NotNil(t, inv.Payment.Terms)
	require.Len(t, inv.Payment.Terms.DueDates, 1)
	require.NotNil(t, inv.Payment.Terms.DueDates[0].Date)
	assert.Equal(t, "2024-06-15", inv.Payment.Terms.DueDates[0].Date.String())

	require.NotNil(t, inv.Payment.Instructions)
	require.Len(t, inv.Payment.Instructions.CreditTransfer, 1)
	assert.Equal(t, "DABADKKK", inv.Payment.Instructions.CreditTransfer[0].BIC)
}
