package addon

import (
	"github.com/invopop/gobl/bill"
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

// validPaymentMeansCodes are the UNTDID 4461 means accepted for OIOUBL (F-LIB100).
// EN 16931's credit-transfer code "30" is absent on purpose: normalizePayInstructions
// rewrites it to OIOUBL's "31" during calculation, before this rule runs, so the
// value seen here is always "31". "42" is excluded: OIOUBL settles it via the
// DK:BANK channel with a branch number + domestic account (F-LIB127/128/131/311),
// which the converter's IBAN mapping can't produce — re-add once DK:BANK modelling
// exists.

// validDocumentTypes are the UNTDID 1001 codes OIOUBL accepts: {325, 380, 393} on
// the Invoice root and 381 on the CreditNote (F-INV011 / F-CRN011). en16931 stamps
// 384/383/389 (corrective/debit-note/self-billed), which OIOUBL rejects — in Denmark
// those are modelled as credit notes.
var validDocumentTypes = []cbc.Code{"325", "380", "381", "393"}

var validPaymentMeansCodes = []cbc.Code{
	"1", "10", "20", "31", "48", "49", "50", "58", "59", "93", "97",
}

// Rule citations reference the OIOUBL Invoice schematron (F-INV) first and the
// CreditNote equivalent (F-CRN) second. F-INV142 is invoice-only (OIOUBL CreditNote
// uses BillingReference, not OrderLineReference).
//
// Deliberately NOT enforced: F-LIB318 (unit code must be in OIOUBL's UN/ECE Rec 20
// subset). The ~1100-code allowlist is a codelist-value check that belongs in
// gobl.ubl, not here; phive rejects an out-of-list unit downstream.

var (
	roundingMin = num.MakeAmount(-1000, 2)
	roundingMax = num.MakeAmount(1000, 2)
)

// forbidWhen builds the "reject the document as soon as a guard matches" rule
// shared by the OIOUBL payment-instruction checks: when cond reports the
// instruction invalid, assert an always-failing test carrying id/msg.
func forbidWhen(desc string, cond func(any) bool, id rules.Code, msg string) rules.Def {
	return rules.When(is.Func(desc, cond),
		rules.Assert(id, msg, is.Func("never", neverTrue)),
	)
}

// positiveAmountRule builds the shared "amount must be greater than zero" field
// rule used by the line/document discount, charge and advance amount checks.
func positiveAmountRule(id rules.Code, msg string) rules.Def {
	return rules.Field("amount",
		rules.Assert(id, msg, is.Func("positive amount", amountPositive)),
	)
}

// billInvoiceRules returns the OIOUBL 2.1 rule set for bill.Invoice
// (covers both invoices and credit notes).
func billInvoiceRules() *rules.Set {
	return rules.For(new(bill.Invoice),
		// OIOUBL relaxes EN 16931 carve-outs that its own schematron does not
		// require: BR-E-10 needs no exemption reason (OIOUBL has no exempt
		// category — exempt is reported as ZeroRated), and BR-CO-25 mandates
		// neither payment means nor terms.
		rules.Ignore(
			"GOBL-EU-EN16931-BILL-INVOICE-06", // BR-CO-25: payment details required
			"GOBL-EU-EN16931-BILL-INVOICE-07", // BR-CO-25: payment terms required
			"GOBL-EU-EN16931-BILL-INVOICE-08", // BR-E-10: exemption reason required
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
			rules.Assert("01", "supplier must have an endpoint or inbox (F-INV031 / F-CRN028)",
				is.Func("has endpoint or inbox", partyHasEndpointOrInbox)),
			rules.Assert("29", "supplier requires a legal identity or a Danish tax ID for the OIOUBL PartyLegalEntity/CompanyID (F-LIB187)",
				is.Func("has an OIOUBL legal company ID", partyHasOIOUBLLegalID)),
		),
		rules.Field("totals",
			// OIOUBL rejects negative payable/due totals outright (F-LIB016 /
			// F-LIB020): over-discounted or over-advanced documents must be
			// modelled as credit notes instead.
			rules.Assert("26", "payable and due totals must not be negative (F-LIB016 / F-LIB020)",
				is.Func("non-negative totals", totalsNonNegative)),
		),
		rules.Field("customer",
			rules.Assert("02", "customer must have an endpoint or inbox (F-INV044 / F-CRN040)",
				is.Func("has endpoint or inbox", partyHasEndpointOrInbox)),
			rules.Assert("30", "customer requires a legal identity or a Danish tax ID for the OIOUBL PartyLegalEntity/CompanyID (F-LIB187)",
				is.Func("has an OIOUBL legal company ID", partyHasOIOUBLLegalID)),
			// F-INV046 requires exactly one Contact in OIOUBL output;
			// gobl.ubl picks one Person at emit time, so the addon asserts presence only.
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
		// EU VAT Directive 2006/112/EC art. 230 requires the VAT amount in the
		// national currency (DKK). A foreign-currency document makes gobl.ubl emit
		// cbc:TaxCurrencyCode, which then obliges a
		// cac:TaxSubtotal/cbc:TransactionCurrencyTaxAmount stating the tax in DKK;
		// that amount can only be derived from an exchange rate, so without one the
		// OIOUBL output fails F-INV018 / F-CRN013.
		rules.When(is.Func("foreign currency without an exchange rate to the regime currency", foreignCurrencyMissingExchangeRate),
			rules.Field("exchange_rates",
				rules.Assert("38", "a foreign-currency document requires an exchange rate to the regime currency (DKK) so VAT is stated in the national currency per EU VAT Directive 2006/112/EC art. 230 (F-INV018 / F-CRN013)", is.Func("never", neverTrue)),
			),
		),
		rules.Field("totals",
			rules.Field("rounding",
				rules.AssertIfPresent("08", "rounding must be between -10.00 and 10.00 (F-INV338 / F-CRN208)", is.Func("in rounding range", roundingInRange)),
			),
		),
		// F-INV239 / F-CRN158: gobl.ubl emits cac:DeliveryLocation whenever
		// delivery.receiver is set; the schematron then requires either an ID
		// (sourced from delivery.identities[0].code) or an Address (sourced
		// from receiver.addresses).
		rules.Field("delivery",
			rules.When(is.Func("receiver set without identities or addresses", deliveryReceiverWithoutLocationData),
				rules.Assert("11", "delivery requires either identities or receiver.addresses (F-INV239 / F-CRN158)", is.Func("never", neverTrue)),
			),
		),
		rules.Field("payment",
			rules.Field("instructions",
				rules.Field("ext",
					rules.AssertIfPresent("12", "payment-means code must be one of the OIOUBL allowed values (F-LIB100)",
						tax.ExtensionsHasCodes(untdid.ExtKeyPaymentMeans, validPaymentMeansCodes...)),
				),
				forbidWhen("bank-transfer payment means without a payee account", bankTransferMissingAccount,
					"13", "a credit transfer account (IBAN or number) is required for bank-transfer payment means (F-LIB107 / F-LIB126)"),
				forbidWhen("iban bank-transfer credit transfer without a BIC", ibanTransferMissingBIC,
					"18", "a BIC is required on the credit transfer for IBAN bank-transfer payment means 30/31 (F-LIB113)"),
				forbidWhen("giro payment means without a 7-8 digit payee account", giroAccountInvalid,
					"21", "Giro (payment-means 50) requires a 7 or 8 digit payee account (F-LIB319 / F-LIB320 / F-LIB321)"),
				forbidWhen("fik payment means without an 8-character creditor account", fikAccountInvalid,
					"22", "FIK (payment-means 93) requires an 8-character creditor account (F-LIB305)"),
			),
		),
		rules.Field("lines",
			rules.Each(
				// Every OIOUBL line needs a cac:TaxTotal (F-INV138 / F-CRN081),
				// which the converter emits only from a VAT combo. GOBL core and
				// en16931 require doc-level totals but not per-line taxes.
				rules.Assert("27", "each line requires a VAT tax category (F-INV138 / F-CRN081)",
					bill.RequireLineTaxCategory(tax.CategoryVAT)),
				rules.Field("quantity",
					rules.Assert("06", "line quantity must not be zero (F-INV147 / F-CRN088)", is.Func("non-zero amount", quantityNonZero)),
				),
				rules.Field("discounts",
					rules.Each(positiveAmountRule("09", "line discount amount must be greater than zero (F-LIB019)")),
				),
				rules.Field("charges",
					rules.Each(positiveAmountRule("10", "line charge amount must be greater than zero (F-LIB019)")),
				),
			),
		),
		// A document-level discount emits cac:AllowanceCharge, whose cbc:Amount
		// must be greater than zero (F-LIB019). A zero-amount allowance —
		// including one promoted from a zero line discount — is wire-fatal.
		rules.Field("discounts",
			rules.Each(positiveAmountRule("33", "document-level discount amount must be greater than zero (F-LIB019)")),
		),
		// A document-level charge emits cac:AllowanceCharge, which OIOUBL
		// requires to carry a TaxCategory (F-LIB226) and a cbc:Amount greater
		// than zero (F-LIB019). en16931 mandates taxes on document-level
		// discounts (BR-32) but not on charges (BR-36 only covers the
		// reason/type), so the taxes rule closes that gap.
		rules.Field("charges",
			rules.Each(
				positiveAmountRule("34", "document-level charge amount must be greater than zero (F-LIB019)"),
				rules.Field("taxes",
					rules.Assert("28", "document-level charge taxes are required for the OIOUBL TaxCategory (F-LIB226)", is.Present),
				),
			),
		),
		// A prepaid advance emits cac:PrepaidPayment/cbc:PaidAmount, which must
		// be greater than zero (F-LIB013).
		rules.Field("payment",
			rules.Field("advances",
				rules.Each(positiveAmountRule("35", "advance amount must be greater than zero (F-LIB013)")),
			),
		),
	)
}

// billTaxComboRules returns the OIOUBL 2.1 rule set applied to every tax combo
// (line- and document-level), validated by type the way GOBL validates combos.
func billTaxComboRules() *rules.Set {
	return rules.For(new(tax.Combo),
		// The OIOUBL category is mapped from the GOBL key, so the dk-oioubl
		// normalizer strips the EN 16931 UNTDID tax-category extension. Ignore the
		// EN 16931 rules that would otherwise require it and validate its code.
		rules.Ignore("GOBL-EU-EN16931-TAX-COMBO-01", "GOBL-EU-EN16931-TAX-COMBO-02"),
		rules.Assert("01", "standard-rated VAT must have a percent greater than zero (F-LIB382)",
			is.Func("standard-rated has a positive percent", standardRatedHasPositivePercent)),
		rules.Assert("32", "VAT category has no OIOUBL equivalent: only standard-rated, zero-rated, exempt and reverse-charge are supported (F-LIB309)",
			is.Func("VAT category maps to an OIOUBL category", vatCategoryHasOIOUBLMapping)),
	)
}

// billChargeRules and lineChargeRules require an excise duty charge (one whose
// Key is a numeric OIOUBL taxschemeid duty code) to carry a reason: the gobl.ubl
// serializer emits it as the OIOUBL tax-scheme name, which OIOUBL requires to be
// non-empty (F-LIB066).
func billChargeRules() *rules.Set { return rules.For(new(bill.Charge), exciseReasonAssert()) }

func lineChargeRules() *rules.Set { return rules.For(new(bill.LineCharge), exciseReasonAssert()) }

// exciseReasonAssert is the shared F-LIB066 assertion for document- and
// line-level charges (which bind to different types and so need separate sets).
func exciseReasonAssert() rules.Def {
	return rules.Assert("01", "an OIOUBL excise duty charge must have a reason for the tax-scheme name (F-LIB066)",
		is.Func("excise charge has a reason", exciseChargeHasReason))
}

// exciseChargeHasReason reports whether an excise duty charge (one whose Key is a
// numeric OIOUBL taxschemeid code) also carries a reason. Ordinary charges (a
// non-numeric key, or none) always pass.
func exciseChargeHasReason(val any) bool {
	var key cbc.Key
	var reason string
	switch c := val.(type) {
	case *bill.Charge:
		if c == nil {
			return true
		}
		key, reason = c.Key, c.Reason
	case *bill.LineCharge:
		if c == nil {
			return true
		}
		key, reason = c.Key, c.Reason
	default:
		return true
	}
	if !isExciseKey(key) {
		return true
	}
	return reason != ""
}

// isExciseKey reports whether a charge Key is an OIOUBL taxschemeid duty code.
// Excise duties are keyed with their numeric code; GOBL's own charge keys are
// alphabetic slugs (stamp-duty, handling, …), so an all-digit key marks the duty.
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

// vatCategoryHasOIOUBLMapping reports whether a VAT combo's GOBL key maps to an
// OIOUBL taxcategoryid-1.1 value. OIOUBL supports only standard, zero (and exempt
// as ZeroRated) and reverse-charge; export/intra-community/outside-scope have no
// equivalent and would reach the wire outside the codelist (F-LIB309).
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

func foreignCurrencyMissingExchangeRate(val any) bool {
	inv, ok := val.(*bill.Invoice)
	if !ok || inv == nil {
		return false
	}
	rd := inv.RegimeDef()
	if rd == nil || inv.Currency == currency.CodeEmpty || inv.Currency == rd.Currency {
		return false
	}
	// Only StandardRated VAT is restated in the national currency, so the rate is
	// obligatory only when such VAT is present; a purely zero-rated/exempt foreign
	// invoice carries no VAT to express in DKK.
	if !invoiceHasStandardRatedVAT(inv) {
		return false
	}
	return currency.MatchExchangeRate(inv.ExchangeRates, inv.Currency, rd.Currency) == nil
}

// invoiceHasStandardRatedVAT reports whether the invoice carries a StandardRated
// VAT combo (GOBL key "standard").
func invoiceHasStandardRatedVAT(inv *bill.Invoice) bool {
	if inv.Totals == nil || inv.Totals.Taxes == nil {
		return false
	}
	for _, cat := range inv.Totals.Taxes.Categories {
		if cat.Code != tax.CategoryVAT {
			continue
		}
		for _, r := range cat.Rates {
			if r.Key == tax.KeyStandard {
				return true
			}
		}
	}
	return false
}

func quantityNonZero(val any) bool {
	a := extractAmount(val)
	return a == nil || !a.IsZero()
}

// amountPositive backs the allowance, charge and advance amount rules: OIOUBL
// rejects a zero or negative cbc:Amount on a cac:AllowanceCharge (F-LIB019) and
// cbc:PaidAmount on a cac:PrepaidPayment (F-LIB013). Rejecting zero is safe —
// corrections are modelled as credit notes, so negative line amounts don't arise.
func amountPositive(val any) bool {
	a := extractAmount(val)
	return a == nil || a.IsPositive()
}

func deliveryReceiverWithoutLocationData(val any) bool {
	del, ok := val.(*bill.DeliveryDetails)
	if !ok || del == nil || del.Receiver == nil {
		return false
	}
	for _, id := range del.Identities {
		if !id.Code.IsEmpty() {
			return false
		}
	}
	return len(del.Receiver.Addresses) == 0
}

func neverTrue(any) bool {
	return false
}

// extractCombo and extractAmount unwrap the value/pointer forms GOBL passes to a
// rule test into a single pointer (nil when the value is neither), so the
// predicates can share one nil-tolerant path instead of repeating the type switch.
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
// identity code, mapped to the OIOUBL cac:Contact/cbc:ID (mandatory for the
// customer, F-INV051). An empty people set passes (rule 03 governs presence).
func firstPersonHasIdentityCode(val any) bool {
	people, ok := val.([]*org.Person)
	if !ok || len(people) == 0 {
		return true
	}
	p := people[0]
	return p != nil && len(p.Identities) > 0 && !p.Identities[0].Code.IsEmpty()
}

// partyHasOIOUBLLegalID reports whether a party can produce a non-empty OIOUBL
// PartyLegalEntity/CompanyID (F-LIB187). The converter emits a PartyLegalEntity for
// any named party, filling CompanyID from the first legal-scope identity or (for a
// Danish party) the CVR; a named non-Danish party with no legal identity would emit
// an empty CompanyID, which OIOUBL rejects.
func partyHasOIOUBLLegalID(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return true
	}
	// No registration name means no PartyLegalEntity is emitted, so F-LIB187
	// cannot fire (the party name is separately required by EN 16931).
	if p.Name == "" {
		return true
	}
	// normalizeParty derives a legal-scope identity from a Danish CVR, so a DK
	// party is covered by the identity check below.
	for _, id := range p.Identities {
		if id.Scope == org.IdentityScopeLegal && !id.Code.IsEmpty() {
			return true
		}
	}
	return false
}

// ibanTransferMissingBIC reports whether an IBAN bank-transfer (means 30→31, or 31)
// carries a credit transfer with no BIC. OIOUBL requires FinancialInstitution/ID
// for the IBAN channel (F-LIB113), which the converter sources from the BIC. Rule
// 13 handles a missing account.
func ibanTransferMissingBIC(val any) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil {
		return false
	}
	if !instr.Ext.Get(untdid.ExtKeyPaymentMeans).In("30", "31") {
		return false
	}
	ct := firstCreditTransfer(instr)
	return ct != nil && ct.BIC == ""
}

// firstCreditTransfer returns the credit transfer the converter emits (the
// first); OIOUBL carries only one, so the payment rules must validate that one.
func firstCreditTransfer(instr *pay.Instructions) *pay.CreditTransfer {
	if len(instr.CreditTransfer) == 0 {
		return nil
	}
	return instr.CreditTransfer[0]
}

// standardRatedHasPositivePercent reports whether a tax combo that maps to the
// OIOUBL StandardRated category (GOBL VAT key "standard") carries a percent
// greater than zero. OIOUBL rejects StandardRated with a zero or absent percent
// (F-LIB382).
func standardRatedHasPositivePercent(val any) bool {
	combo := extractCombo(val)
	if combo == nil || combo.Key != tax.KeyStandard {
		return true
	}
	return combo.Percent != nil && !combo.Percent.Base().IsZero() && !combo.Percent.Base().IsNegative()
}

// bankTransferCodes are the OIOUBL PaymentMeansCode values that settle to a
// payee bank account: 31 (IBAN), 30 (generic credit transfer, which the gobl.ubl
// converter maps to 31), and 58 (SEPA credit transfer). OIOUBL then requires the
// account identifier (F-LIB107 for 30/31, F-LIB377 for 58), which GOBL core
// leaves optional. (42 is excluded — see validPaymentMeansCodes.)
var bankTransferCodes = []cbc.Code{"30", "31", "58"}

func bankTransferMissingAccount(val any) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil {
		return false
	}
	code := instr.Ext.Get(untdid.ExtKeyPaymentMeans)
	if !code.In(bankTransferCodes...) {
		return false
	}
	ct := firstCreditTransfer(instr)
	return ct == nil || (ct.IBAN == "" && ct.Number == "")
}



// giroAccountInvalid reports whether a Giro (payment-means 50) instruction's
// payee account is missing or not 7-8 numeric digits (F-LIB319/320/321).
func giroAccountInvalid(val any) bool {
	return accountLengthInvalid(val, "50", isGiroAccountNumber)
}

// fikAccountInvalid reports whether a FIK (payment-means 93) instruction's
// creditor account is missing or not exactly 8 characters (F-LIB305).
func fikAccountInvalid(val any) bool {
	return accountLengthInvalid(val, "93", func(s string) bool { return len(s) == 8 })
}

// accountLengthInvalid fires when the instruction uses the given payment-means
// code but no credit transfer carries an account number satisfying ok.
func accountLengthInvalid(val any, code cbc.Code, ok func(string) bool) bool {
	instr, isInstr := val.(*pay.Instructions)
	if !isInstr || instr == nil {
		return false
	}
	if instr.Ext.Get(untdid.ExtKeyPaymentMeans) != code {
		return false
	}
	ct := firstCreditTransfer(instr)
	return ct == nil || !ok(ct.Number)
}






// isNumericOfLen reports whether s consists only of ASCII digits and has a
// length within [minLen, maxLen].
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
