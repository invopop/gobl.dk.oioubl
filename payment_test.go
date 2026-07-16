package dkoioubl_test

import (
	"path/filepath"
	"testing"

	dkoioubl "github.com/invopop/gobl.dk.oioubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/pay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	t.Run("DK:BANK carries the reg. nr. and never a BIC", func(t *testing.T) {
		env := loadTestEnvelope(t, filepath.Join(getConvertPath(), "dk-bank.json"))

		// A BIC-shaped value on the GOBL side must not survive: for means 42 the
		// flat branch ID is the reg. nr. from the branch label (F-LIB124/130).
		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		inv.Payment.Instructions.CreditTransfer[0].BIC = "DABADKKK"

		doc, err := dkoioubl.ConvertInvoice(env)
		require.NoError(t, err)
		require.NotEmpty(t, doc.PaymentMeans)
		pm := doc.PaymentMeans[0]
		assert.Equal(t, "42", pm.PaymentMeansCode.Value)
		require.NotNil(t, pm.PaymentChannelCode)
		assert.Equal(t, "DK:BANK", pm.PaymentChannelCode.Value)
		require.NotNil(t, pm.PayeeFinancialAccount)
		require.NotNil(t, pm.PayeeFinancialAccount.ID)
		assert.Equal(t, "0440116243", *pm.PayeeFinancialAccount.ID)
		branch := pm.PayeeFinancialAccount.FinancialInstitutionBranch
		require.NotNil(t, branch)
		require.NotNil(t, branch.ID)
		assert.Equal(t, "1234", *branch.ID)
		assert.Nil(t, branch.FinancialInstitution)

		data, err := dkoioubl.Bytes(doc)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "DABADKKK")
	})

	t.Run("NemKonto keeps only the means and channel codes", func(t *testing.T) {
		env := loadTestEnvelope(t, filepath.Join(getConvertPath(), "nemkonto.json"))

		// Account and reference details on the GOBL side must not leak onto the
		// means: NemKonto allows none of them (F-LIB159 – F-LIB165).
		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		inv.Payment.Instructions.Ref = "12345678"
		inv.Payment.Instructions.CreditTransfer = []*pay.CreditTransfer{
			{Number: "0440116243", BIC: "DABADKKK"},
		}

		doc, err := dkoioubl.ConvertInvoice(env)
		require.NoError(t, err)
		require.NotEmpty(t, doc.PaymentMeans)
		pm := doc.PaymentMeans[0]
		assert.Equal(t, "97", pm.PaymentMeansCode.Value)
		require.NotNil(t, pm.PaymentChannelCode)
		assert.Equal(t, "DK:NEMKONTO", pm.PaymentChannelCode.Value)
		assert.Nil(t, pm.PayeeFinancialAccount)
		assert.Nil(t, pm.PayerFinancialAccount)
		assert.Nil(t, pm.CreditAccount)
		assert.Nil(t, pm.PaymentID)
		assert.Nil(t, pm.InstructionID)
		assert.Empty(t, pm.InstructionNote)
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
