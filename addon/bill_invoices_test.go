package addon_test

import (
	"testing"

	_ "github.com/invopop/gobl"
	oioubl "github.com/invopop/gobl.dk.oioubl/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/tax"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDutyCode(code cbc.Code) tax.Extensions {
	return tax.ExtensionsOf(cbc.CodeMap{oioubl.ExtKeyDutyCode: code})
}

func testInvoiceStandard(t *testing.T) *bill.Invoice {
	t.Helper()
	return &bill.Invoice{
		Regime:    tax.WithRegime("DK"),
		Addons:    tax.WithAddons(oioubl.V2),
		IssueDate: cal.MakeDate(2026, 1, 1),
		Type:      "standard",
		Currency:  "DKK",
		Series:    "2026",
		Code:      "1000",
		Supplier: &org.Party{
			Name: "Eksempel A/S",
			TaxID: &tax.Identity{
				Country: "DK",
				Code:    "12345674",
			},
			Inboxes: []*org.Inbox{
				{Scheme: "DK:CVR", Code: "12345674"},
			},
			Addresses: []*org.Address{
				{Number: "1", Street: "Hovedgaden", Locality: "København", Code: "1000", Country: "DK"},
			},
		},
		Customer: &org.Party{
			Name: "Kunde ApS",
			TaxID: &tax.Identity{
				Country: "DK",
				Code:    "88146328",
			},
			Inboxes: []*org.Inbox{
				{Scheme: "DK:CVR", Code: "88146328"},
			},
			Addresses: []*org.Address{
				{Number: "5", Street: "Bygaden", Locality: "Aarhus", Code: "8000", Country: "DK"},
			},
		},
		Lines: []*bill.Line{
			{
				Quantity: num.MakeAmount(1, 0),
				Item: &org.Item{
					Name:  "Produkt",
					Price: num.NewAmount(10000, 2),
				},
				Taxes: tax.Set{
					{Category: "VAT", Rate: "standard"},
				},
			},
		},
	}
}

func TestInvoiceValidation(t *testing.T) {
	t.Run("standard invoice", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})

	t.Run("unsupported document type is rejected (F-INV011)", func(t *testing.T) {
		// 384 (corrective) is outside the OIOUBL invoicetypecode-1.1 set; in
		// Denmark corrections are modelled as credit notes.
		inv := testInvoiceStandard(t)
		require.NoError(t, inv.Calculate())
		inv.Tax.Ext = inv.Tax.Ext.Set(untdid.ExtKeyDocumentType, "384")
		assert.ErrorContains(t, rules.Validate(inv), "F-INV011")
	})

	t.Run("line without a VAT tax is rejected (F-INV138 / F-CRN081)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Lines[0].Taxes = nil
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "VAT")
	})

	t.Run("document-level charge without taxes is rejected (F-LIB226)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Charges = []*bill.Charge{
			{Reason: "Freight", Amount: num.MakeAmount(10000, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB226")
	})

	t.Run("document-level charge with a VAT tax passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Charges = []*bill.Charge{
			{Reason: "Freight", Amount: num.MakeAmount(10000, 2),
				Taxes: tax.Set{{Category: "VAT", Rate: "standard"}}},
		}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("excise duty charge without a reason is rejected (F-LIB066)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Lines[0].Charges = []*bill.LineCharge{
			{Key: oioubl.ChargeKeyExcise, Ext: testDutyCode("16"), Amount: num.MakeAmount(1000, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB066")
	})

	t.Run("excise duty charge with a reason passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Lines[0].Charges = []*bill.LineCharge{
			{Key: oioubl.ChargeKeyExcise, Ext: testDutyCode("16"), Reason: "Mineralvandsafgift", Amount: num.MakeAmount(1000, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("excise duty charge without a duty code is rejected", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Lines[0].Charges = []*bill.LineCharge{
			{Key: oioubl.ChargeKeyExcise, Reason: "Mineralvandsafgift", Amount: num.MakeAmount(1000, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "duty code")
	})

	t.Run("document-level excise duty without a VAT tax is rejected", func(t *testing.T) {
		// Danish car registration tax: zero-rated, diverging from the
		// standard-rated line it applies to, so it must be document-level —
		// and, since it diverges, its VAT type can't be inferred.
		inv := testInvoiceStandard(t)
		inv.Charges = []*bill.Charge{
			{Key: oioubl.ChargeKeyExcise, Ext: testDutyCode("66"), Reason: "Registreringsafgift", Amount: num.MakeAmount(10000000, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "requires a VAT tax")
	})

	t.Run("document-level excise duty with a VAT tax passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Charges = []*bill.Charge{
			{
				Key:    oioubl.ChargeKeyExcise,
				Ext:    testDutyCode("66"),
				Reason: "Registreringsafgift",
				Amount: num.MakeAmount(10000000, 2),
				Taxes:  tax.Set{{Category: "VAT", Key: "zero"}},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("document-level excise duty without a duty code is rejected", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Charges = []*bill.Charge{
			{
				Key:    oioubl.ChargeKeyExcise,
				Reason: "Registreringsafgift",
				Amount: num.MakeAmount(10000000, 2),
				Taxes:  tax.Set{{Category: "VAT", Key: "zero"}},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "duty code")
	})

	t.Run("line-level excise duty does not require its own VAT tax", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Lines[0].Charges = []*bill.LineCharge{
			{Key: oioubl.ChargeKeyExcise, Ext: testDutyCode("16"), Reason: "Mineralvandsafgift", Amount: num.MakeAmount(1000, 2)},
		}
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})

	t.Run("default currency rounding rule passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		require.NoError(t, inv.Calculate())
		assert.Equal(t, tax.RoundingRuleCurrency, inv.Tax.Rounding)
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("explicit override away from currency rounding fails (F-INV126)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Tax = &bill.Tax{Rounding: tax.RoundingRulePrecise}
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv)
		assert.ErrorContains(t, err, "F-INV126")
	})

}

func testCreditNoteStandard(t *testing.T) *bill.Invoice {
	t.Helper()
	inv := testInvoiceStandard(t)
	inv.Type = bill.InvoiceTypeCreditNote
	inv.Code = "CN-1000"
	return inv
}

func TestCreditNoteValidation(t *testing.T) {
	t.Run("standard credit note", func(t *testing.T) {
		cn := testCreditNoteStandard(t)
		require.NoError(t, cn.Calculate())
		require.NoError(t, rules.Validate(cn))
	})
}
