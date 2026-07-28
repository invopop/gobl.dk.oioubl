package addon

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/cef"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// validDocumentTypes are the UNTDID 1001 codes OIOUBL accepts: 325/380/393
// (invoice) and 381 (credit note), per F-INV011 / F-CRN011.
var validDocumentTypes = []cbc.Code{"325", "380", "381", "393"}

// Rule citations reference the OIOUBL Invoice schematron (F-INV) first and the
// CreditNote equivalent (F-CRN) second. F-INV142 is invoice-only (OIOUBL CreditNote
// uses BillingReference, not OrderLineReference).

// Deliberately NOT enforced: F-LIB318 (unit code must be in OIOUBL's UN/ECE Rec 20
// subset) ~1100-code list.

var (
	roundingMin = num.MakeAmount(-1000, 2)
	roundingMax = num.MakeAmount(1000, 2)
)

func billInvoiceRules() *rules.Set {
	return rules.For(new(bill.Invoice),
		// Ignore EN 16931 BR-CO-25 (06/07, payment means/terms — OIOUBL doesn't need
		// them) and BR-E-10 (08 — inert once we strip its ext; rule 39 re-adds it).
		rules.Ignore(
			"GOBL-EU-EN16931-BILL-INVOICE-06", // BR-CO-25: payment details required
			"GOBL-EU-EN16931-BILL-INVOICE-07", // BR-CO-25: payment terms required
			"GOBL-EU-EN16931-BILL-INVOICE-08", // BR-E-10: re-enforced by rule 39
		),
		rules.Field("code",
			rules.Assert("05", "invoice code is required (F-INV009 / F-CRN006)", is.Present),
		),
		rules.Field("tax",
			rules.Field("ext",
				rules.AssertIfPresent("31", "document type must be an OIOUBL-supported code: invoice 325/380/393 or credit note 381 (F-INV011)",
					tax.ExtensionsHasCodes(untdid.ExtKeyDocumentType, validDocumentTypes...)),
			),
			rules.Assert("41", "OIOUBL requires GOBL's currency rounding rule. (F-INV128 / F-INV133)",
				is.Func("uses currency rounding", taxUsesCurrencyRounding)),
		),
		rules.Field("supplier",
			rules.Assert("01", "the supplier must have an endpoint (F-INV031 / F-CRN028)",
				is.Func("has endpoint", partyHasEndpoint)),
			rules.Assert("29", "the supplier must have an official company ID, such as a CVR number (F-INV034/F-CRN031)",
				is.Func("has an OIOUBL legal company ID", partyHasOIOUBLLegalID)),
		),
		rules.Field("totals",
			rules.Assert("26", "payable and due totals must not be negative (F-LIB016 / F-LIB020)",
				is.Func("non-negative totals", totalsNonNegative)),
		),
		rules.Field("customer",
			rules.Assert("02", "the customer must have an endpoint (F-INV044 / F-CRN040)",
				is.Func("has endpoint", partyHasEndpoint)),
			rules.Field("people",
				rules.Assert("03", "customer people are required (F-INV046 / F-CRN042)", is.Present),
				rules.Assert("20", "the customer contact person requires an identity code for the OIOUBL Contact/ID (F-INV051)",
					is.Func("first person has an identity code", firstPersonHasIdentityCode)),
			),
		),
		rules.When(is.Func("non-credit-note invoice with line order ref", invoiceWithLineOrderRef),
			rules.Field("ordering",
				rules.Assert("07", "ordering is required when any invoice line has an order reference (F-INV142)", is.Present),
			),
		),

		rules.Assert("38", "a foreign-currency document requires an exchange rate to the regime currency so VAT is stated in the national currency per EU VAT Directive 2006/112/EC art. 230 (F-INV018 / F-CRN013)",
			is.Func("foreign-currency VAT restated in the regime currency", foreignCurrencyExchangeRateOK)),
		// OIOUBL maps exempt to ZeroRated; en16931's BR-E-10 keys on the UNTDID
		// tax-category ext the normalizer strips, so re-assert on the exempt key.
		rules.Assert("39", "an exempt VAT category requires a CEF VATEX exemption reason for the OIOUBL cbc:TaxExemptionReasonCode (BR-E-10)",
			is.Func("exempt VAT has a VATEX reason", exemptVATHasReason)),
		rules.Field("totals",
			rules.Field("rounding",
				rules.AssertIfPresent("08", "rounding must be between -10.00 and 10.00 (F-INV338 / F-CRN208)", is.Func("in rounding range", roundingInRange)),
			),
		),

		rules.Field("delivery",
			rules.Assert("11", "delivery requires either identities or receiver.addresses (F-INV239 / F-CRN158)",
				is.Func("receiver has identities or addresses", deliveryReceiverHasLocationData)),
		),
		rules.Field("lines",
			rules.Each(
				rules.Assert("27", "each line requires a VAT tax category (F-INV138 / F-CRN081)",
					bill.RequireLineTaxCategory(tax.CategoryVAT)),
				rules.Field("quantity",
					rules.Assert("06", "line quantity must not be zero (F-INV147 / F-CRN088)", is.Func("non-zero amount", quantityNonZero)),
				),
				rules.Field("discounts",
					rules.Each(rules.Field("amount", rules.Assert("09", "line discount amount must be greater than zero (F-LIB019)", num.Positive))),
				),
				rules.Field("charges",
					rules.Each(rules.Field("amount", rules.Assert("10", "line charge amount must be greater than zero (F-LIB019)", num.Positive))),
				),
			),
		),

		rules.Field("discounts",
			rules.Each(rules.Field("amount", rules.Assert("33", "document-level discount amount must be greater than zero (F-LIB019)", num.Positive))),
		),

		rules.Field("charges",
			rules.Each(
				rules.Field("amount", rules.Assert("34", "document-level charge amount must be greater than zero (F-LIB019)", num.Positive)),
				// Same taxesHaveVAT check as rule 02, just for non-excise charges (mutually exclusive with it) with a distinct citation.
				rules.When(is.Func("non-excise charge", chargeIsNotExcise),
					rules.Field("taxes",
						rules.Assert("28", "document-level charge requires a VAT tax for the OIOUBL TaxCategory (F-LIB226)",
							is.Func("has a VAT combo", taxesHaveVAT)),
					),
				),
			),
		),
		// OIOUBL requires a prepaid advance's cbc:PaidAmount to be greater than
		// zero (F-LIB013).
		rules.Field("payment",
			rules.Field("advances",
				rules.Each(rules.Field("amount", rules.Assert("35", "advance amount must be greater than zero (F-LIB013)", num.Positive))),
			),
		),
	)
}

// billTaxComboRules validates every VAT tax.Combo in the document.
func billTaxComboRules() *rules.Set {
	return rules.For(new(tax.Combo),
		// OIOUBL uses its own taxcategoryid, so the normalizer drops the UNTDID
		// tax-category ext; skip the EN 16931 rules that require and code-check it.
		rules.Ignore("GOBL-EU-EN16931-TAX-COMBO-01", "GOBL-EU-EN16931-TAX-COMBO-02"),
		rules.Assert("01", "standard-rated VAT must have a percent greater than zero (F-LIB382)",
			is.Func("standard-rated has a positive percent", standardRatedHasPositivePercent)),
		rules.Assert("32", "VAT category has no OIOUBL equivalent: only standard-rated, zero-rated, exempt and reverse-charge are supported (F-LIB309)",
			is.Func("VAT category maps to an OIOUBL category", vatCategoryHasOIOUBLMapping)),
	)
}

func billChargeRules() *rules.Set {
	return rules.For(new(bill.Charge), exciseReasonAssert(), exciseDutyVATTaxAssert(), exciseDutyCodeAssert())
}

func lineChargeRules() *rules.Set {
	return rules.For(new(bill.LineCharge), exciseReasonAssert(), exciseDutyCodeAssert())
}

func exciseReasonAssert() rules.Def {
	return rules.When(is.Func("excise duty charge", chargeIsExcise),
		rules.Field("reason",
			rules.Assert("01", "an OIOUBL excise duty charge requires a reason for its tax-scheme name (F-LIB066)", is.Present),
		),
	)
}

func exciseDutyVATTaxAssert() rules.Def {
	return rules.When(is.Func("excise duty charge", chargeIsExcise),
		rules.Field("taxes",
			rules.Assert("02", "a document-level OIOUBL excise duty requires a VAT tax stating its own VAT type (OIOUBL Skat guideline)",
				is.Func("has a VAT combo", taxesHaveVAT)),
		),
	)
}

func exciseDutyCodeAssert() rules.Def {
	return rules.When(is.Func("excise duty charge", chargeIsExcise),
		rules.Field("ext",
			rules.Assert("03", "an OIOUBL excise duty charge requires the SKAT duty code extension for its tax-scheme ID",
				tax.ExtensionsRequire(ExtKeyDutyCode)),
		),
	)
}

func taxUsesCurrencyRounding(val any) bool {
	t, ok := val.(*bill.Tax)
	if !ok || t == nil {
		return true
	}
	return t.Rounding == "" || t.Rounding == tax.RoundingRuleCurrency
}

func totalsNonNegative(val any) bool {
	tt, ok := val.(*bill.Totals)
	if !ok || tt == nil {
		return true
	}
	if tt.Payable.IsNegative() {
		return false
	}
	return tt.Due == nil || !tt.Due.IsNegative()
}

func invoiceWithLineOrderRef(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	if inv.Type == bill.InvoiceTypeCreditNote {
		return false
	}
	for _, line := range inv.Lines {
		if !line.Order.IsEmpty() {
			return true
		}
	}
	return false
}

// foreignCurrencyExchangeRateOK reports whether a foreign-currency invoice with
// VAT to restate carries an exchange rate to the regime currency (art. 230).
func foreignCurrencyExchangeRateOK(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return true
	}
	rd := inv.RegimeDef()
	needsRate := rd != nil &&
		inv.Currency != currency.CodeEmpty &&
		inv.Currency != rd.Currency &&
		invoiceHasPositiveVAT(inv)
	if needsRate {
		return currency.MatchExchangeRate(inv.ExchangeRates, inv.Currency, rd.Currency) != nil
	}
	return true
}

// exemptVATHasReason reports whether every exempt VAT rate carries a CEF VATEX
// code or an exemption note (mirroring en16931's BR-E-10).
func exemptVATHasReason(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Totals == nil || inv.Totals.Taxes == nil {
		return true
	}
	hasExemptNote := false
	if inv.Tax != nil {
		for _, n := range inv.Tax.Notes {
			if n != nil && n.Key == tax.KeyExempt && n.Text != "" {
				hasExemptNote = true
				break
			}
		}
	}
	for _, cat := range inv.Totals.Taxes.Categories {
		if cat.Code != tax.CategoryVAT {
			continue
		}
		for _, r := range cat.Rates {
			if r.Key == tax.KeyExempt && r.Ext.Get(cef.ExtKeyVATEX).IsEmpty() && !hasExemptNote {
				return false
			}
		}
	}
	return true
}

func roundingInRange(val any) bool {
	a := extractAmount(val)
	return a == nil || (a.Compare(roundingMin) >= 0 && a.Compare(roundingMax) <= 0)
}

func deliveryReceiverHasLocationData(val any) bool {
	del, ok := val.(*bill.DeliveryDetails)
	if !ok || del == nil || del.Receiver == nil {
		return true
	}
	for _, id := range del.Identities {
		if id != nil && !id.Code.IsEmpty() {
			return true
		}
	}
	return len(del.Receiver.Addresses) > 0
}

func quantityNonZero(val any) bool {
	a := extractAmount(val)
	return a == nil || !a.IsZero()
}

func taxesHaveVAT(val any) bool {
	set, ok := val.(tax.Set)
	if !ok {
		return true
	}
	return set.Get(tax.CategoryVAT) != nil
}

func chargeIsNotExcise(val any) bool {
	return !chargeIsExcise(val)
}

func standardRatedHasPositivePercent(val any) bool {
	combo := extractCombo(val)
	if combo == nil || combo.Category != tax.CategoryVAT || combo.Key != tax.KeyStandard {
		return true
	}
	return combo.Percent != nil && combo.Percent.Base().IsPositive()
}

// vatCategoryHasOIOUBLMapping reports whether a VAT combo's key maps to an OIOUBL
// taxcategoryid-1.1 value (standard/zero/reverse-charge, exempt folds into zero).
func vatCategoryHasOIOUBLMapping(val any) bool {
	combo := extractCombo(val)
	if combo == nil || combo.Category != tax.CategoryVAT {
		return true
	}
	return taxCategoryMapsToOIOUBL(combo.Key)
}

// chargeIsExcise reports whether a charge is an OIOUBL excise duty.
func chargeIsExcise(val any) bool {
	switch c := val.(type) {
	case *bill.Charge:
		return c != nil && c.Key == ChargeKeyExcise
	case *bill.LineCharge:
		return c != nil && c.Key == ChargeKeyExcise
	}
	return false
}

func invoiceHasPositiveVAT(inv *bill.Invoice) bool {
	if inv.Totals == nil || inv.Totals.Taxes == nil {
		return false
	}
	for _, cat := range inv.Totals.Taxes.Categories {
		if cat.Code == tax.CategoryVAT && cat.Amount.IsPositive() {
			return true
		}
	}
	return false
}

// extractCombo/extractAmount take a value that may arrive as a copy or a
// pointer, and always hand back a pointer.
func extractCombo(val any) *tax.Combo {
	switch c := val.(type) {
	case *tax.Combo:
		return c
	case tax.Combo:
		return &c
	}
	return nil
}

func extractAmount(val any) *num.Amount {
	switch a := val.(type) {
	case num.Amount:
		return &a
	case *num.Amount:
		return a
	}
	return nil
}

// taxCategoryMapsToOIOUBL reports whether a GOBL VAT key has an OIOUBL
// taxcategoryid-1.1 equivalent (standard/zero/reverse-charge, exempt folds into zero).
func taxCategoryMapsToOIOUBL(key cbc.Key) bool {
	switch key {
	case tax.KeyStandard, tax.KeyZero, tax.KeyExempt, tax.KeyReverseCharge, "":
		return true
	}
	return false
}

// normalizeInvoice defaults to GOBL's currency rounding rule to match OIOUBL's
// own rounding (F-INV128/F-INV133).
func normalizeInvoice(inv *bill.Invoice) {
	if inv.Tax == nil {
		inv.Tax = new(bill.Tax)
	}
	if inv.Tax.Rounding == "" {
		inv.Tax.Rounding = tax.RoundingRuleCurrency
	}
}
