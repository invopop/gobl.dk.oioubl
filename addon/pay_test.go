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

	t.Run("non-OIOUBL payment-means code fails (F-LIB100)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key: pay.MeansKeyOther,
				Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "57"}),
			},
		}
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv)
		assert.ErrorContains(t, err, "F-LIB100")
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

	t.Run("domestic bank transfer 42 with a non-numeric reg. nr. fails (F-LIB130)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key: pay.MeansKeyOther,
				Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "42"}),
				CreditTransfer: []*pay.CreditTransfer{
					{Number: "0440116243", Name: "DABADKKK"},
				},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB130")
	})

	t.Run("domestic bank transfer 42 without an account number fails (F-LIB126)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key: pay.MeansKeyOther,
				Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "42"}),
				CreditTransfer: []*pay.CreditTransfer{
					{Name: "1234"},
				},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB126")
	})

	t.Run("SEPA credit-transfer code 58 with account passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key:            pay.MeansKeyOther,
				Ext:            tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "58"}),
				CreditTransfer: []*pay.CreditTransfer{{IBAN: "DK5000400440116243", BIC: "DABADKKK"}},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("SEPA credit-transfer code 58 without account fails (F-LIB377)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key: pay.MeansKeyOther,
				Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "58"}),
			},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB377")
	})

	t.Run("bank-transfer code 31 without account fails (F-LIB107)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key:            pay.MeansKeyOther,
				Ext:            tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "31"}),
				CreditTransfer: []*pay.CreditTransfer{{Name: "Bank, no account number"}},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB107")
	})

	t.Run("bank-transfer code 31 with account only on a non-first transfer fails (F-LIB107)", func(t *testing.T) {
		// The converter emits only the first credit transfer, so an account on a
		// later entry doesn't satisfy the requirement.
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key:            pay.MeansKeyOther,
				Ext:            tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "31"}),
				CreditTransfer: []*pay.CreditTransfer{{Name: "no account"}, {IBAN: "DK5000400440116243", BIC: "DABADKKK"}},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB107")
	})

	t.Run("bank-transfer code 31 without a BIC fails (F-LIB113)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key:            pay.MeansKeyOther,
				Ext:            tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "31"}),
				CreditTransfer: []*pay.CreditTransfer{{IBAN: "DK5000400440116243"}},
			},
		}
		require.NoError(t, inv.Calculate())
		// F-LIB113 requires the FinancialInstitution/ID (sourced from the BIC) on
		// the IBAN channel; only the 2017 $IbanOnly variant is commented out.
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB113")
	})

	t.Run("bank-transfer code 31 with no credit transfer at all fails (F-LIB107)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key: pay.MeansKeyOther,
				Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "31"}),
			},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB107")
	})

	t.Run("NemKonto code 97 without a credit transfer passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key: pay.MeansKeyOther,
				Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "97"}),
			},
		}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("NemKonto code 97 with a credit transfer fails (F-LIB164)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key:            pay.MeansKeyOther,
				Ext:            tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "97"}),
				CreditTransfer: []*pay.CreditTransfer{{IBAN: "DK5000400440116243", BIC: "DABADKKK"}},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB164")
	})

	t.Run("NemKonto code 97 with a payment reference fails (F-LIB161)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key: pay.MeansKeyOther,
				Ext: tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "97"}),
				Ref: "12345678",
			},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB161")
	})

	t.Run("NemKonto code 97 with a direct debit fails (F-LIB163)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key:         pay.MeansKeyOther,
				Ext:         tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "97"}),
				DirectDebit: &pay.DirectDebit{Account: "DK5000400440116243"},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB163")
	})

	t.Run("FIK code 93 with a non-8-character account fails (F-LIB305)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Instructions: &pay.Instructions{
				Key:            pay.MeansKeyOther,
				Ext:            tax.ExtensionsOf(cbc.CodeMap{untdid.ExtKeyPaymentMeans: "93"}),
				CreditTransfer: []*pay.CreditTransfer{{Number: "123"}},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB305")
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
