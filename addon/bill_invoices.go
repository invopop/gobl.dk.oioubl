package addon

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/cef"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// validDocumentTypes are the UNTDID 1001 codes OIOUBL accepts: 325/380/393
// (invoice) and 381 (credit note), per F-INV011 / F-CRN011.
var validDocumentTypes = []cbc.Code{"325", "380", "381", "393"}

// validPaymentMeansCodes are the UNTDID 4461 means accepted for OIOUBL (F-LIB100).
var validPaymentMeansCodes = []cbc.Code{
	"1", "10", "20", "31", "48", "49", "50", "58", "59", "93", "97",
}

// Rule citations reference the OIOUBL Invoice schematron (F-INV) first and the
// CreditNote equivalent (F-CRN) second. F-INV142 is invoice-only (OIOUBL CreditNote
// uses BillingReference, not OrderLineReference).
// Reference: https://git.erst.dk/openebusiness/common/-/tree/master/resources/Schematrons/OIOUBL?ref_type=heads

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
		),
		rules.Field("supplier",
			rules.Assert("01", "supplier must have an endpoint (F-INV031 / F-CRN028)",
				is.Func("has endpoint", partyHasEndpoint)),
			rules.Assert("29", "supplier requires a legal identity or a Danish tax ID for the OIOUBL PartyLegalEntity/CompanyID (F-LIB187)",
				is.Func("has an OIOUBL legal company ID", partyHasOIOUBLLegalID)),
		),
		rules.Field("totals",
			rules.Assert("26", "payable and due totals must not be negative (F-LIB016 / F-LIB020)",
				is.Func("non-negative totals", totalsNonNegative)),
		),
		rules.Field("customer",
			rules.Assert("02", "customer must have an endpoint (F-INV044 / F-CRN040)",
				is.Func("has endpoint", partyHasEndpoint)),
			rules.Assert("30", "customer requires a legal identity or a Danish tax ID for the OIOUBL PartyLegalEntity/CompanyID (F-LIB187)",
				is.Func("has an OIOUBL legal company ID", partyHasOIOUBLLegalID)),
			// F-INV046 requires a customer Contact (F-CRN042); assert presence.
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
		rules.Field("payment",
			rules.Field("instructions",
				rules.Field("ext",
					rules.AssertIfPresent("12", "payment-means code must be one of the OIOUBL allowed values (F-LIB100)",
						tax.ExtensionsHasCodes(untdid.ExtKeyPaymentMeans, validPaymentMeansCodes...)),
				),
				rules.Assert("13", "a credit transfer account (IBAN or number) is required for bank-transfer payment means (F-LIB107 / F-LIB377)",
					is.Func("bank-transfer has a payee account", bankTransferHasAccount)),
				rules.Assert("18", "a BIC is required on the credit transfer for IBAN bank-transfer payment means 30/31 (F-LIB113)",
					is.Func("iban bank-transfer has a BIC", ibanTransferHasBIC)),
				rules.Assert("21", "Giro (payment-means 50) requires a 7 or 8 digit payee account (F-LIB319 / F-LIB320 / F-LIB321)",
					is.Func("giro has a 7-8 digit payee account", giroAccountValid)),
				rules.Assert("22", "FIK (payment-means 93) requires an 8-character creditor account (F-LIB305)",
					is.Func("fik has an 8-character creditor account", fikAccountValid)),
			),
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
				rules.Field("taxes",
					rules.Assert("28", "document-level charge taxes are required for the OIOUBL TaxCategory (F-LIB226)", is.Present),
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

// billTaxComboRules validates every VAT tax.Combo in the document — GOBL applies
// type-scoped rules to all of them (invoice lines, charges, discounts).
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

// Excise duty charges must carry a reason for their OIOUBL tax-scheme name (F-LIB066).
func billChargeRules() *rules.Set { return rules.For(new(bill.Charge), exciseReasonAssert()) }

func lineChargeRules() *rules.Set { return rules.For(new(bill.LineCharge), exciseReasonAssert()) }

// exciseReasonAssert is the shared F-LIB066 rule for document- and line-level
// charges (which bind to different types and so need separate sets).
func exciseReasonAssert() rules.Def {
	return rules.When(is.Func("excise duty charge", chargeIsExcise),
		rules.Field("reason",
			rules.Assert("01", "an OIOUBL excise duty charge requires a reason for its tax-scheme name (F-LIB066)", is.Present),
		),
	)
}

// chargeIsExcise reports whether a charge is an OIOUBL excise duty, keyed by an
// all-digit taxschemeid code.
func chargeIsExcise(val any) bool {
	switch c := val.(type) {
	case *bill.Charge:
		return c != nil && isExciseKey(c.Key)
	case *bill.LineCharge:
		return c != nil && isExciseKey(c.Key)
	}
	return false
}

// isExciseKey reports whether a charge Key is an OIOUBL excise duty code: all digits.
func isExciseKey(key cbc.Key) bool {
	s := key.String()
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// vatCategoryHasOIOUBLMapping reports whether a VAT combo's key maps to an OIOUBL
// taxcategoryid-1.1 value (standard/zero/exempt/reverse-charge); others fail F-LIB309.
func vatCategoryHasOIOUBLMapping(val any) bool {
	combo := extractCombo(val)
	if combo == nil || combo.Category != tax.CategoryVAT {
		return true
	}
	return taxCategoryMapsToOIOUBL(combo.Key)
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

// exemptVATHasReason reports whether every exempt VAT rate carries a CEF VATEX
// reason (keyed on the GOBL exempt key, which survives normalization).
func exemptVATHasReason(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil || inv.Totals == nil || inv.Totals.Taxes == nil {
		return true
	}
	for _, cat := range inv.Totals.Taxes.Categories {
		if cat.Code != tax.CategoryVAT {
			continue
		}
		for _, r := range cat.Rates {
			if r.Key == tax.KeyExempt && r.Ext.Get(cef.ExtKeyVATEX).IsEmpty() {
				return false
			}
		}
	}
	return true
}

func quantityNonZero(val any) bool {
	a := extractAmount(val)
	return a == nil || !a.IsZero()
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

// extractCombo/extractAmount normalize the argument a GOBL rule test receives —
// which may be the value (tax.Combo) or a pointer (*tax.Combo) — to one pointer
// (nil if neither), so a predicate handles both forms without its own type switch.
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

// firstPersonHasIdentityCode reports whether the first contact person carries an
// identity code, mapped to the OIOUBL cac:Contact/cbc:ID
func firstPersonHasIdentityCode(val any) bool {
	people, ok := val.([]*org.Person)
	if !ok || len(people) == 0 {
		return true
	}
	p := people[0]
	return p != nil && len(p.Identities) > 0 && p.Identities[0] != nil && !p.Identities[0].Code.IsEmpty()
}

// partyHasEndpoint reports whether a party carries a NemHandel endpoint (BT-34 /
// cbc:EndpointID); the normalizer derives one from a DK tax ID or scheme inbox.
func partyHasEndpoint(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return true
	}
	return len(p.Endpoints) > 0
}

// partyHasOIOUBLLegalID reports whether a named party can produce a non-empty
// OIOUBL PartyLegalEntity/CompanyID from a legal identity or Danish CVR (F-LIB187).
func partyHasOIOUBLLegalID(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return true
	}
	// A party with no name has no PartyLegalEntity, so F-LIB187 (its CompanyID) can't apply.
	if p.Name == "" {
		return true
	}
	for _, id := range p.Identities {
		if id != nil && id.Scope == org.IdentityScopeLegal && !id.Code.IsEmpty() {
			return true
		}
	}
	return false
}

func ibanTransferHasBIC(val any) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil {
		return true
	}
	if !instr.Ext.Get(untdid.ExtKeyPaymentMeans).In("30", "31") {
		return true
	}
	ct := firstCreditTransfer(instr)
	// A missing account is rule 13's concern (F-LIB113 covers only the BIC).
	return ct == nil || ct.BIC != ""
}

// firstCreditTransfer returns the first credit transfer; OIOUBL carries only
// one, so the payment rules validate that one.
func firstCreditTransfer(instr *pay.Instructions) *pay.CreditTransfer {
	if len(instr.CreditTransfer) == 0 {
		return nil
	}
	return instr.CreditTransfer[0]
}

// standardRatedHasPositivePercent reports whether a standard-rated VAT combo has
// a percent greater than zero; OIOUBL rejects a zero or absent percent (F-LIB382).
func standardRatedHasPositivePercent(val any) bool {
	combo := extractCombo(val)
	if combo == nil || combo.Key != tax.KeyStandard {
		return true
	}
	return combo.Percent != nil && !combo.Percent.Base().IsZero() && !combo.Percent.Base().IsNegative()
}

// bankTransferCodes are the OIOUBL PaymentMeansCode values requiring a payee
// account (F-LIB107 for 30/31, F-LIB377 for 58); 30 is normalized to 31.
var bankTransferCodes = []cbc.Code{"30", "31", "58"}

func bankTransferHasAccount(val any) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil {
		return true
	}
	code := instr.Ext.Get(untdid.ExtKeyPaymentMeans)
	if !code.In(bankTransferCodes...) {
		return true
	}
	ct := firstCreditTransfer(instr)
	return ct != nil && (ct.IBAN != "" || ct.Number != "")
}

// giroAccountValid checks F-LIB319/320/321: a Giro payment (means 50) must
// carry a payee account number of 7 or 8 digits.
func giroAccountValid(val any) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil || instr.Ext.Get(untdid.ExtKeyPaymentMeans) != "50" {
		return true
	}
	ct := firstCreditTransfer(instr)
	return ct != nil && isGiroAccountNumber(ct.Number)
}

// fikAccountValid checks F-LIB305: a FIK payment (means 93) must carry an
// 8-character creditor account number.
func fikAccountValid(val any) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil || instr.Ext.Get(untdid.ExtKeyPaymentMeans) != "93" {
		return true
	}
	ct := firstCreditTransfer(instr)
	return ct != nil && len(ct.Number) == 8
}

func isNumericOfLen(s string, minLen, maxLen int) bool {
	if len(s) < minLen || len(s) > maxLen {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isGiroAccountNumber(s string) bool {
	return isNumericOfLen(s, 7, 8)
}

func roundingInRange(val any) bool {
	a := extractAmount(val)
	return a == nil || (a.Compare(roundingMin) >= 0 && a.Compare(roundingMax) <= 0)
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
