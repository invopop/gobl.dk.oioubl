package addon_test

import (
	"testing"

	_ "github.com/invopop/gobl"
	oioubl "github.com/invopop/gobl.dk.oioubl/addon"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/catalogues/cef"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
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
			People: []*org.Person{
				{
					Name:       &org.Name{Given: "Anders", Surname: "Jensen"},
					Identities: []*org.Identity{{Label: "Contact", Code: "C-001"}},
				},
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

	t.Run("document-level charge with an empty (non-nil) tax set is rejected (F-LIB226)", func(t *testing.T) {
		// Rule 28 requires a VAT combo specifically (same check as the excise
		// VAT-tax requirement, rule 02), not just a non-nil Taxes slice -- an
		// empty slice is present but has no VAT combo, so it must still fail.
		inv := testInvoiceStandard(t)
		inv.Charges = []*bill.Charge{
			{Reason: "Freight", Amount: num.MakeAmount(10000, 2), Taxes: tax.Set{}},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB226")
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

	t.Run("excise duty charge with a non-numeric duty code passes", func(t *testing.T) {
		// The SKAT codelist has non-numeric codes (e.g. "21d").
		inv := testInvoiceStandard(t)
		inv.Lines[0].Charges = []*bill.LineCharge{
			{Key: oioubl.ChargeKeyExcise, Ext: testDutyCode("21d"), Reason: "CO2-afgift", Amount: num.MakeAmount(1000, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
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

	t.Run("foreign-currency document without an exchange rate is rejected (F-INV018)", func(t *testing.T) {
		// A non-DKK document must restate VAT in the regime currency (F-INV018),
		// which requires an exchange rate.
		inv := testInvoiceStandard(t)
		inv.Currency = "EUR"
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-INV018")
	})

	t.Run("foreign-currency document with an exchange rate passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Currency = "EUR"
		inv.ExchangeRates = []*currency.ExchangeRate{
			{From: "EUR", To: "DKK", Amount: num.MakeAmount(745, 2)},
		}
		require.NoError(t, inv.Calculate())
		if err := rules.Validate(inv); err != nil {
			assert.NotContains(t, err.Error(), "F-INV018")
		}
	})

	t.Run("ordering present with only accounting cost is allowed", func(t *testing.T) {
		// OIOUBL F-INV024 only constrains cac:OrderReference/ID; an accounting
		// cost emits cbc:AccountingCost, not an OrderReference, so no code is
		// required here.
		inv := testInvoiceStandard(t)
		inv.Ordering = &bill.Ordering{Cost: "5050"}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("missing invoice code fails (F-INV009)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Code = ""
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv)
		assert.ErrorContains(t, err, "F-INV009")
	})

	t.Run("zero line quantity fails (F-INV147)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Lines[0].Quantity = num.MakeAmount(0, 0)
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv)
		assert.ErrorContains(t, err, "F-INV147")
	})

	t.Run("line order ref without invoice ordering fails (F-INV142)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Lines[0].Order = "PO-LINE-1"
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv)
		assert.ErrorContains(t, err, "F-INV142")
	})

	t.Run("line order ref with invoice ordering passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Lines[0].Order = "PO-LINE-1"
		inv.Ordering = &bill.Ordering{Code: "PO-2026-001"}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("rounding above 10.00 fails (F-INV338)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		require.NoError(t, inv.Calculate())
		excess := num.MakeAmount(1500, 2)
		inv.Totals.Rounding = &excess
		err := rules.Validate(inv)
		assert.ErrorContains(t, err, "F-INV338")
	})

	t.Run("rounding below -10.00 fails (F-INV338)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		require.NoError(t, inv.Calculate())
		excess := num.MakeAmount(-1500, 2)
		inv.Totals.Rounding = &excess
		err := rules.Validate(inv)
		assert.ErrorContains(t, err, "F-INV338")
	})

	t.Run("rounding within range passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		require.NoError(t, inv.Calculate())
		amount := num.MakeAmount(500, 2)
		inv.Totals.Rounding = &amount
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("default currency rounding rule passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		require.NoError(t, inv.Calculate())
		assert.Equal(t, tax.RoundingRuleCurrency, inv.Tax.Rounding)
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("explicit override away from currency rounding fails (F-INV128)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Tax = &bill.Tax{Rounding: tax.RoundingRulePrecise}
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv)
		assert.ErrorContains(t, err, "F-INV128")
	})

	t.Run("zero line discount fails (F-LIB019)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Lines[0].Discounts = []*bill.LineDiscount{
			{Reason: "Loyalty", Amount: num.MakeAmount(0, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB019")
	})

	t.Run("negative line discount fails (F-LIB019)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Lines[0].Discounts = []*bill.LineDiscount{
			{Reason: "Loyalty", Amount: num.MakeAmount(-500, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB019")
	})

	t.Run("positive line discount passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Lines[0].Discounts = []*bill.LineDiscount{
			{Reason: "Loyalty", Amount: num.MakeAmount(500, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("zero line charge fails (F-LIB019)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Lines[0].Charges = []*bill.LineCharge{
			{Reason: "Handling", Amount: num.MakeAmount(0, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB019")
	})

	t.Run("negative line charge fails (F-LIB019)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Lines[0].Charges = []*bill.LineCharge{
			{Reason: "Handling", Amount: num.MakeAmount(-500, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB019")
	})

	t.Run("zero document-level discount fails (F-LIB019)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Discounts = []*bill.Discount{
			{Reason: "Goodwill", Amount: num.MakeAmount(0, 2)},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB019")
	})

	t.Run("zero document-level charge fails (F-LIB019)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Charges = []*bill.Charge{
			{Reason: "Freight", Amount: num.MakeAmount(0, 2),
				Taxes: tax.Set{{Category: "VAT", Rate: "standard"}}},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB019")
	})

	t.Run("zero advance fails (F-LIB013)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Payment = &bill.PaymentDetails{
			Advances: []*pay.Record{
				{Description: "Deposit", Amount: num.MakeAmount(0, 2)},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB013")
	})

	t.Run("tax category without an OIOUBL mapping is rejected (F-LIB309)", func(t *testing.T) {
		// Intra-community (K), export (G) and not-subject (O) have no OIOUBL
		// taxcategoryid-1.1 equivalent; in Denmark these are not OIOUBL traffic.
		inv := testInvoiceStandard(t)
		require.NoError(t, inv.Calculate())
		inv.Lines[0].Taxes[0].Key = tax.KeyIntraCommunity
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB309")
	})

	t.Run("delivery with receiver and addresses passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Delivery = &bill.DeliveryDetails{
			Receiver: &org.Party{
				Name: "Modtager A/S",
				Addresses: []*org.Address{
					{Street: "Leveringsvej 2", Locality: "Odense", Code: "5000", Country: "DK"},
				},
			},
		}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("delivery with receiver only and no identities fails (F-INV239)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Delivery = &bill.DeliveryDetails{
			Receiver: &org.Party{Name: "Modtager A/S"},
		}
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv)
		assert.ErrorContains(t, err, "F-INV239")
	})

	t.Run("delivery with receiver and identities passes (no addresses)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Delivery = &bill.DeliveryDetails{
			Receiver:   &org.Party{Name: "Modtager A/S"},
			Identities: []*org.Identity{{Code: "DEL-LOC-1"}},
		}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("inline street address with no separate number passes (StructuredLax)", func(t *testing.T) {
		// OIOUBL addresses are StructuredLax: no mandatory sub-fields (only
		// F-LIB036 forbids free-text AddressLine), so an inline house number or
		// missing postcode is valid; EN 16931 BR-8/BR-10 still govern presence
		// and country.
		inv := testInvoiceStandard(t)
		inv.Supplier.Addresses = []*org.Address{
			{Street: "Hovedgaden 27", Locality: "København", Country: "DK"},
		}
		require.NoError(t, inv.Calculate())
		require.NoError(t, rules.Validate(inv))
	})

	t.Run("standard-rated VAT with zero percent fails (F-LIB382)", func(t *testing.T) {
		// The OIOUBL StandardRated category is derived from the eu-en16931
		// untdid-tax-category extension, so this rule is exercised in the real
		// [eu-en16931-v2017, dk-oioubl-v2] stack that DK invoices always use.
		inv := testInvoiceStandard(t)
		inv.Addons = tax.WithAddons("eu-en16931-v2017", oioubl.V2)
		zero := num.MakePercentage(0, 3)
		inv.Lines[0].Taxes = tax.Set{{Category: "VAT", Key: "standard", Percent: &zero}}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "F-LIB382")
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

	t.Run("missing credit-note code fails (F-CRN006)", func(t *testing.T) {
		cn := testCreditNoteStandard(t)
		cn.Code = ""
		require.NoError(t, cn.Calculate())
		err := rules.Validate(cn)
		assert.ErrorContains(t, err, "F-CRN006")
	})

	t.Run("zero credit-note line quantity fails (F-CRN088)", func(t *testing.T) {
		cn := testCreditNoteStandard(t)
		cn.Lines[0].Quantity = num.MakeAmount(0, 0)
		require.NoError(t, cn.Calculate())
		err := rules.Validate(cn)
		assert.ErrorContains(t, err, "F-CRN088")
	})

	t.Run("credit note with line order ref does not fire F-INV142", func(t *testing.T) {
		cn := testCreditNoteStandard(t)
		cn.Lines[0].Order = "PO-LINE-1"
		require.NoError(t, cn.Calculate())
		assert.NoError(t, rules.Validate(cn))
	})

	t.Run("zero credit-note line discount fails (F-LIB019)", func(t *testing.T) {
		cn := testCreditNoteStandard(t)
		cn.Lines[0].Discounts = []*bill.LineDiscount{
			{Reason: "Loyalty", Amount: num.MakeAmount(0, 2)},
		}
		require.NoError(t, cn.Calculate())
		assert.ErrorContains(t, rules.Validate(cn), "F-LIB019")
	})

	t.Run("negative credit-note line discount fails (F-LIB019)", func(t *testing.T) {
		cn := testCreditNoteStandard(t)
		cn.Lines[0].Discounts = []*bill.LineDiscount{
			{Reason: "Loyalty", Amount: num.MakeAmount(-500, 2)},
		}
		require.NoError(t, cn.Calculate())
		assert.ErrorContains(t, rules.Validate(cn), "F-LIB019")
	})
}

func TestTotalsNonNegative(t *testing.T) {
	t.Run("over-discounted invoice is rejected (F-LIB016)", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Discounts = []*bill.Discount{{
			Reason: "Goodwill",
			Amount: num.MakeAmount(1000000, 2),
			Taxes:  tax.Set{{Category: tax.CategoryVAT, Key: tax.KeyStandard, Percent: num.NewPercentage(250, 3)}},
		}}
		require.NoError(t, inv.Calculate())
		err := rules.Validate(inv)
		assert.ErrorContains(t, err, "F-LIB016")
	})

	t.Run("fully discounted to zero passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		require.NoError(t, inv.Calculate())
		inv.Discounts = []*bill.Discount{{
			Reason: "Goodwill",
			Amount: inv.Totals.Total,
			Taxes:  tax.Set{{Category: tax.CategoryVAT, Key: tax.KeyStandard, Percent: num.NewPercentage(250, 3)}},
		}}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})
}

// TestInvoiceEN16931Relaxations covers the EN 16931 over-enforcement rules that
// gobl core relaxes (or keeps) only when the dk-oioubl addon is present: the
// production rules gate on the addon key, so the behaviour is verified here.
func TestInvoiceEN16931Relaxations(t *testing.T) {
	t.Run("exempt without a reason is rejected (BR-E-10 kept)", func(t *testing.T) {
		// exempt maps to OIOUBL ZeroRated, but the exemption reason must accompany
		// it so an exempt supply stays distinguishable from a true zero-rated one,
		// so BR-E-10 is enforced.
		inv := testInvoiceStandard(t)
		inv.Lines[0].Taxes = tax.Set{{Category: tax.CategoryVAT, Key: tax.KeyExempt}}
		require.NoError(t, inv.Calculate())
		assert.ErrorContains(t, rules.Validate(inv), "VATEX exemption reason")
	})

	t.Run("exempt with a VATEX reason passes", func(t *testing.T) {
		inv := testInvoiceStandard(t)
		inv.Lines[0].Taxes = tax.Set{{
			Category: tax.CategoryVAT,
			Key:      tax.KeyExempt,
			Ext:      tax.ExtensionsOf(cbc.CodeMap{cef.ExtKeyVATEX: "VATEX-EU-132"}),
		}}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})

	t.Run("exempt with a note instead of a VATEX code passes", func(t *testing.T) {
		// Mirrors en16931's own VATEX-or-note allowance (BR-E-10); the note's
		// GOBL key survives normalization even though its UNTDID ext doesn't.
		inv := testInvoiceStandard(t)
		inv.Lines[0].Taxes = tax.Set{{Category: tax.CategoryVAT, Key: tax.KeyExempt}}
		inv.Tax = &bill.Tax{
			Notes: []*tax.Note{{Key: tax.KeyExempt, Text: "Exempt under local rules"}},
		}
		require.NoError(t, inv.Calculate())
		assert.NoError(t, rules.Validate(inv))
	})
}
