package addon_test

import (
	"testing"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func bankPayment() *bill.PaymentDetails {
	return &bill.PaymentDetails{
		Terms: &pay.Terms{Notes: "Net 30 days"},
		Instructions: &pay.Instructions{
			// MeansKeyDebitTransfer maps to UNTDID code 31, the only OIOUBL-valid
			// bank-transfer code; MeansKeyCreditTransfer (code 30) is rejected.
			Key:            pay.MeansKeyDebitTransfer,
			CreditTransfer: []*pay.CreditTransfer{{IBAN: "DK5000400440116243", BIC: "DABADKKK"}},
		},
	}
}

func TestPaymentValidation(t *testing.T) {
	t.Run("OIOUBL payment-means code 31 passes (F-LIB100)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key: pay.MeansKeyOther,
				Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "31"}),
				CreditTransfer: []*pay.CreditTransfer{
					{IBAN: "DK5000400440116243", BIC: "DABADKKK"},
				},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("generic credit-transfer code 30 is rejected (F-LIB100)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key:            pay.MeansKeyOther,
				Ext:            tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "30"}),
				CreditTransfer: []*pay.CreditTransfer{{IBAN: "DK5000400440116243", BIC: "DABADKKK"}},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB100")
	})

	t.Run("domestic bank transfer 42 with account and reg. nr. passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key: pay.MeansKeyOther,
				Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "42"}),
				CreditTransfer: []*pay.CreditTransfer{
					{Number: "0440116243", Name: "1234"},
				},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("domestic bank transfer 42 without a reg. nr. fails (F-LIB124)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key:            pay.MeansKeyOther,
				Ext:            tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "42"}),
				CreditTransfer: []*pay.CreditTransfer{{Number: "0440116243"}},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB124")
	})

	t.Run("due invoice without payment skips BR-CO-25", func(t *testing.T) {
		// OIOUBL's payment rules are all conditional-on-presence, so a due
		// invoice with no payment means/terms must not trip EN 16931's BR-CO-25.
		inv := testInvoiceStandard(t)
		inv.Payment = nil
		require.NoError(t, inv.Calculate())
		if err := rules.Validate(inv); err != nil {
			assert.NotContains(t, err.Error(), "payment details are required")
			assert.NotContains(t, err.Error(), "payment terms are required")
		}
	})

	t.Run("amount-only payment terms pass", func(t *testing.T) {
		// OIOUBL allows bare payment terms — its official samples carry terms
		// with only an ID and amount — so EN 16931's due-dates-or-notes shape
		// requirement must not fire.
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Terms: &pay.Terms{},
		}
		require.NoError(t, inv.Calculate())
		if err := rules.Validate(inv); err != nil {
			assert.NotContains(t, err.Error(), "due_dates or notes")
		}
	})
}
