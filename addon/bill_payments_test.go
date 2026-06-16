package addon_test

import (
	"testing"

	_ "github.com/invopop/gobl"
	oioubl "github.com/invopop/gobl.dk.oioubl/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRequestPayment builds a valid OIOUBL Reminder, modelled as a bill.Payment
// of type "request" referencing one outstanding invoice.
func testRequestPayment(t *testing.T) *bill.Payment {
	t.Helper()
	return &bill.Payment{
		Regime:    tax.WithRegime("DK"),
		Addons:    tax.WithAddons(oioubl.V2_1),
		Type:      bill.PaymentTypeRequest,
		IssueDate: cal.MakeDate(2026, 1, 1),
		Currency:  "DKK",
		Series:    "2026",
		Code:      "R-1000",
		Ext: tax.ExtensionsOf(cbc.CodeMap{
			oioubl.ExtKeyReminderType:     oioubl.ExtValueReminderTypeReminder,
			oioubl.ExtKeyReminderSequence: "1",
		}),
		Supplier: &org.Party{
			Name:    "Eksempel A/S",
			TaxID:   &tax.Identity{Country: "DK", Code: "12345674"},
			Inboxes: []*org.Inbox{{Scheme: "0184", Code: "12345674"}},
		},
		Customer: &org.Party{
			Name:    "Kunde ApS",
			TaxID:   &tax.Identity{Country: "DK", Code: "88146328"},
			Inboxes: []*org.Inbox{{Scheme: "0184", Code: "88146328"}},
			People: []*org.Person{
				{Name: &org.Name{Given: "Anders", Surname: "Jensen"}},
			},
		},
		Lines: []*bill.PaymentLine{
			{
				Document: &org.DocumentRef{Code: "1000", IssueDate: cal.NewDate(2025, 12, 1)},
				Amount:   num.MakeAmount(125000, 2),
			},
		},
		Methods: []*pay.Record{
			{Key: "credit-transfer", CreditTransfer: &pay.CreditTransfer{IBAN: "DK5000400440116243", BIC: "DABADKKK"}},
		},
	}
}

func TestPaymentValidation(t *testing.T) {
	t.Run("valid request payment", func(t *testing.T) {
		p := testRequestPayment(t)
		require.NoError(t, p.Calculate())
		require.NoError(t, rules.Validate(p))
	})

	// The OIOUBL Reminder rules are scoped to the "request" type. A receipt
	// payment carries no OIOUBL document, so none of them apply even when the
	// reminder-specific data is absent.
	t.Run("non-request type skips the OIOUBL rules", func(t *testing.T) {
		p := testRequestPayment(t)
		p.Type = bill.PaymentTypeReceipt
		p.Code = ""
		p.Ext = tax.Extensions{}
		p.Customer = nil
		require.NoError(t, p.Calculate())
		require.NoError(t, rules.Validate(p))
	})

	t.Run("code is required (F-REM010)", func(t *testing.T) {
		p := testRequestPayment(t)
		p.Code = ""
		require.NoError(t, p.Calculate())
		assert.ErrorContains(t, rules.Validate(p), "F-REM010")
	})

	t.Run("reminder type is required (F-REM006 / F-REM061)", func(t *testing.T) {
		p := testRequestPayment(t)
		p.Ext = p.Ext.Delete(oioubl.ExtKeyReminderType)
		require.NoError(t, p.Calculate())
		assert.ErrorContains(t, rules.Validate(p), "F-REM006")
	})

	t.Run("reminder sequence is required (F-REM007)", func(t *testing.T) {
		p := testRequestPayment(t)
		p.Ext = p.Ext.Delete(oioubl.ExtKeyReminderSequence)
		require.NoError(t, p.Calculate())
		assert.ErrorContains(t, rules.Validate(p), "F-REM007")
	})

	t.Run("supplier endpoint is required (F-REM018)", func(t *testing.T) {
		p := testRequestPayment(t)
		p.Supplier.Inboxes = nil
		p.Supplier.TaxID = nil
		require.NoError(t, p.Calculate())
		assert.ErrorContains(t, rules.Validate(p), "F-REM018")
	})

	t.Run("supplier legal identity is required (F-REM021)", func(t *testing.T) {
		p := testRequestPayment(t)
		p.Supplier.TaxID = &tax.Identity{Country: "DE", Code: "111111125"}
		p.Supplier.Inboxes = []*org.Inbox{{Scheme: "0088", Code: "4035811991021"}}
		require.NoError(t, p.Calculate())
		assert.ErrorContains(t, rules.Validate(p), "F-REM021")
	})

	t.Run("customer is required (F-REM024)", func(t *testing.T) {
		p := testRequestPayment(t)
		p.Customer = nil
		require.NoError(t, p.Calculate())
		assert.ErrorContains(t, rules.Validate(p), "F-REM024")
	})

	t.Run("customer endpoint is required (F-REM025)", func(t *testing.T) {
		p := testRequestPayment(t)
		p.Customer.Inboxes = nil
		p.Customer.TaxID = nil
		require.NoError(t, p.Calculate())
		assert.ErrorContains(t, rules.Validate(p), "F-REM025")
	})

	t.Run("customer legal identity is required (F-LIB187)", func(t *testing.T) {
		p := testRequestPayment(t)
		p.Customer.TaxID = &tax.Identity{Country: "DE", Code: "111111125"}
		p.Customer.Inboxes = []*org.Inbox{{Scheme: "0088", Code: "4035811991021"}}
		require.NoError(t, p.Calculate())
		assert.ErrorContains(t, rules.Validate(p), "F-LIB187")
	})

	t.Run("customer contact person is required (F-REM071)", func(t *testing.T) {
		p := testRequestPayment(t)
		p.Customer.People = nil
		require.NoError(t, p.Calculate())
		assert.ErrorContains(t, rules.Validate(p), "F-REM071")
	})

	t.Run("payee endpoint is required when a payee is present (F-REM034)", func(t *testing.T) {
		p := testRequestPayment(t)
		p.Payee = &org.Party{Name: "Inkasso A/S"}
		require.NoError(t, p.Calculate())
		assert.ErrorContains(t, rules.Validate(p), "F-REM034")
	})
}
