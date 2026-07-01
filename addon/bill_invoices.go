package addon

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// validPaymentMeansCodes are the UNTDID 4461 means accepted for OIOUBL (F-LIB100).
// "30" is included because the converter maps it to OIOUBL's "31". "42" is excluded:
// OIOUBL settles it via the DK:BANK channel with a branch number + domestic account
// (F-LIB127/128/131/311), which the converter's IBAN mapping can't produce — re-add
// once DK:BANK modelling exists.

// validDocumentTypes are the UNTDID 1001 codes OIOUBL accepts: {325, 380, 393} on
// the Invoice root and 381 on the CreditNote (F-INV011 / F-CRN011). en16931 stamps
// 384/383/389 (corrective/debit-note/self-billed), which OIOUBL rejects — in Denmark
// those are modelled as credit notes.
var validDocumentTypes = []cbc.Code{"325", "380", "381", "393"}

var validPaymentMeansCodes = []cbc.Code{
	"1", "10", "20", "30", "31", "48", "49", "50", "58", "59", "93", "97",
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
				rules.AssertIfPresent("31", "document type must be an OIOUBL-supported code: invoice 325/380/393 or credit note 381 (F-INV011 / F-CRN011)",
					tax.ExtensionsHasCodes(untdid.ExtKeyDocumentType, validDocumentTypes...)),
			),
		),
		rules.Field("supplier",
			rules.Assert("01", "supplier must have an ISO 6523 endpoint or inbox (F-INV031 / F-CRN028)",
				is.Func("has endpoint or inbox", partyHasEndpointOrInbox)),
			rules.Assert("29", "supplier requires a legal identity or a Danish tax ID for the OIOUBL PartyLegalEntity/CompanyID (F-LIB187)",
				is.Func("has an OIOUBL legal company ID", partyHasOIOUBLLegalID)),
		),
		rules.Field("totals",
			// OIOUBL rejects negative monetary totals outright (F-LIB016 on
			// PayableAmount / TaxInclusiveAmount, F-LIB020 on amounts):
			// over-discounted or over-advanced documents must be modelled as
			// credit notes instead.
			rules.Assert("26", "payable and due totals must not be negative (F-LIB016 / F-LIB020)",
				is.Func("non-negative totals", totalsNonNegative)),
		),
		rules.Field("customer",
			rules.Assert("02", "customer must have an ISO 6523 endpoint or inbox (F-INV044 / F-CRN040)",
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
				rules.When(is.Func("bank-transfer payment means without a payee account", bankTransferMissingAccount),
					rules.Assert("13", "a credit transfer account (IBAN or number) is required for bank-transfer payment means (F-LIB107 / F-LIB126)", is.Func("never", neverTrue)),
				),
				rules.When(is.Func("iban bank-transfer credit transfer without a BIC", ibanTransferMissingBIC),
					rules.Assert("18", "a BIC is required on the credit transfer for IBAN bank-transfer payment means 30/31 (F-LIB113)", is.Func("never", neverTrue)),
				),
				rules.When(is.Func("giro payment means without a valid OIOUBL payment id", giroPaymentIDInvalid),
					rules.Assert("14", "Giro (payment-means 50) requires a dk-oioubl-payment-id of 01, 04 or 15 (F-LIB144 / F-LIB147)", is.Func("never", neverTrue)),
				),
				rules.When(is.Func("fik payment means without a valid OIOUBL payment id", fikPaymentIDInvalid),
					rules.Assert("15", "FIK (payment-means 93) requires a dk-oioubl-payment-id of 71, 73 or 75 (F-LIB152)", is.Func("never", neverTrue)),
				),
				rules.When(is.Func("structured giro/fik payment id without a valid reference", structuredPaymentRefInvalid),
					rules.Assert("23", "structured Giro/FIK payment id (04/15/71/75) requires a numeric payment reference of the required length (F-LIB145 / F-LIB153 / F-LIB156 / F-LIB157 / F-LIB312 / F-LIB336)", is.Func("never", neverTrue)),
				),
				rules.When(is.Func("fik kortart 73 carrying a payment reference", fik73WithReference),
					rules.Assert("24", "FIK payment id 73 must not carry a payment reference, OIOUBL has no element for it (F-LIB275)", is.Func("never", neverTrue)),
				),
				rules.When(is.Func("giro kortart 01 with an over-long payment reference", giro01ReferenceTooLong),
					rules.Assert("25", "Giro payment id 01 payment reference must be at most 16 characters (F-LIB149)", is.Func("never", neverTrue)),
				),
				rules.When(is.Func("giro payment means without a 7-8 digit payee account", giroAccountInvalid),
					rules.Assert("21", "Giro (payment-means 50) requires a 7 or 8 digit payee account (F-LIB319 / F-LIB320 / F-LIB321)", is.Func("never", neverTrue)),
				),
				rules.When(is.Func("fik payment means without an 8-character creditor account", fikAccountInvalid),
					rules.Assert("22", "FIK (payment-means 93) requires an 8-character creditor account (F-LIB305)", is.Func("never", neverTrue)),
				),
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
					rules.Each(
						rules.Field("amount",
							rules.Assert("09", "line discount amount must be greater than zero (F-LIB019)", is.Func("positive amount", amountPositive)),
						),
					),
				),
				rules.Field("charges",
					rules.Each(
						rules.Field("amount",
							rules.Assert("10", "line charge amount must be greater than zero (F-LIB019)", is.Func("positive amount", amountPositive)),
						),
					),
				),
			),
		),
		// A document-level discount emits cac:AllowanceCharge, whose cbc:Amount
		// must be greater than zero (F-LIB019). A zero-amount allowance —
		// including one promoted from a zero line discount — is wire-fatal.
		rules.Field("discounts",
			rules.Each(
				rules.Field("amount",
					rules.Assert("33", "document-level discount amount must be greater than zero (F-LIB019)", is.Func("positive amount", amountPositive)),
				),
			),
		),
		// A document-level charge emits cac:AllowanceCharge, which OIOUBL
		// requires to carry a TaxCategory (F-LIB226) and a cbc:Amount greater
		// than zero (F-LIB019). en16931 mandates taxes on document-level
		// discounts (BR-32) but not on charges (BR-36 only covers the
		// reason/type), so the taxes rule closes that gap.
		rules.Field("charges",
			rules.Each(
				rules.Field("amount",
					rules.Assert("34", "document-level charge amount must be greater than zero (F-LIB019)", is.Func("positive amount", amountPositive)),
				),
				rules.Field("taxes",
					rules.Assert("28", "document-level charge taxes are required for the OIOUBL TaxCategory (F-LIB226)", is.Present),
				),
			),
		),
		// A prepaid advance emits cac:PrepaidPayment/cbc:PaidAmount, which must
		// be greater than zero (F-LIB013).
		rules.Field("payment",
			rules.Field("advances",
				rules.Each(
					rules.Field("amount",
						rules.Assert("35", "advance amount must be greater than zero (F-LIB013)", is.Func("positive amount", amountPositive)),
					),
				),
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

// billChargeRules and lineChargeRules require an excise duty charge (one tagged
// with the dk-oioubl-tax-scheme extension) to carry a reason: the gobl.ubl
// serializer emits it as the OIOUBL tax-scheme name, which OIOUBL requires to be
// non-empty (F-LIB066).
func billChargeRules() *rules.Set {
	return rules.For(new(bill.Charge),
		rules.Assert("01", "an OIOUBL excise duty charge must have a reason for the tax-scheme name (F-LIB066)",
			is.Func("excise charge has a reason", exciseChargeHasReason)),
	)
}

func lineChargeRules() *rules.Set {
	return rules.For(new(bill.LineCharge),
		rules.Assert("01", "an OIOUBL excise duty charge must have a reason for the tax-scheme name (F-LIB066)",
			is.Func("excise charge has a reason", exciseChargeHasReason)),
	)
}

// exciseChargeHasReason reports whether a charge that carries the OIOUBL excise
// tax-scheme extension also carries a reason. Ordinary charges (no extension)
// always pass.
func exciseChargeHasReason(val any) bool {
	var ext tax.Extensions
	var reason string
	switch c := val.(type) {
	case *bill.Charge:
		if c == nil {
			return true
		}
		ext, reason = c.Ext, c.Reason
	case *bill.LineCharge:
		if c == nil {
			return true
		}
		ext, reason = c.Ext, c.Reason
	default:
		return true
	}
	if ext.Get(ExtKeyTaxScheme).String() == "" {
		return true
	}
	return reason != ""
}

// vatCategoryHasOIOUBLMapping reports whether a VAT combo's GOBL key maps to an
// OIOUBL taxcategoryid-1.1 value. OIOUBL supports only standard, zero (and exempt
// as ZeroRated) and reverse-charge; export/intra-community/outside-scope have no
// equivalent and would reach the wire outside the codelist (F-LIB309).
func vatCategoryHasOIOUBLMapping(val any) bool {
	var combo *tax.Combo
	switch c := val.(type) {
	case *tax.Combo:
		combo = c
	case tax.Combo:
		combo = &c
	default:
		return true
	}
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

func quantityNonZero(val any) bool {
	switch a := val.(type) {
	case num.Amount:
		return !a.IsZero()
	case *num.Amount:
		return a == nil || !a.IsZero()
	}
	return true
}

// amountPositive backs the allowance, charge and advance amount rules: OIOUBL
// rejects a zero or negative cbc:Amount on a cac:AllowanceCharge (F-LIB019) and
// cbc:PaidAmount on a cac:PrepaidPayment (F-LIB013). Rejecting zero is safe —
// corrections are modelled as credit notes, so negative line amounts don't arise.
func amountPositive(val any) bool {
	switch a := val.(type) {
	case num.Amount:
		return a.IsPositive()
	case *num.Amount:
		return a == nil || a.IsPositive()
	}
	return true
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
	// A Danish tax identity is fabricated into the CompanyID as the CVR.
	if p.TaxID != nil && p.TaxID.Country == "DK" && p.TaxID.Code != "" {
		return true
	}
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
	var combo *tax.Combo
	switch c := val.(type) {
	case *tax.Combo:
		combo = c
	case tax.Combo:
		combo = &c
	default:
		return true
	}
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

func giroPaymentIDInvalid(val any) bool {
	return paymentIDInvalidFor(val, "50", giroPaymentIDs)
}

func fikPaymentIDInvalid(val any) bool {
	return paymentIDInvalidFor(val, "93", fikPaymentIDs)
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

// structuredPaymentRefInvalid reports whether a structured Giro/FIK kortart (Giro
// 04/15, FIK 71/75) is missing the numeric cbc:InstructionID reference or has the
// wrong length: F-LIB145/153 (mandatory), F-LIB312/336 (numeric), F-LIB149 (Giro
// ≤16), F-LIB156 (FIK 71 = 15), F-LIB157 (FIK 75 = 16). Simple kortart (Giro 01,
// FIK 73) carry no reference.
func structuredPaymentRefInvalid(val any) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil {
		return false
	}
	means := instr.Ext.Get(untdid.ExtKeyPaymentMeans)
	ref := instr.Ref.String()
	switch instr.Ext.Get(ExtKeyPaymentID) {
	case "04", "15":
		return means == "50" && !isNumericOfLen(ref, 1, 16)
	case "71":
		return means == "93" && !isNumericOfLen(ref, 15, 15)
	case "75":
		return means == "93" && !isNumericOfLen(ref, 16, 16)
	}
	return false
}

// fik73RefForbidden reports whether an OIOUBL payment using FIK kortart 73, which
// forbids cbc:InstructionID, still carries a payment reference that OIOUBL has
// nowhere to put (F-LIB275).
func fik73RefForbidden(ext tax.Extensions, ref string) bool {
	return ext.Get(untdid.ExtKeyPaymentMeans) == "93" &&
		ext.Get(ExtKeyPaymentID) == "73" &&
		ref != ""
}

// fik73WithReference applies fik73RefForbidden to an invoice payment instruction.
func fik73WithReference(val any) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil {
		return false
	}
	return fik73RefForbidden(instr.Ext, instr.Ref.String())
}

// fik73RecordWithReference applies fik73RefForbidden to a reminder payment method.
func fik73RecordWithReference(val any) bool {
	m, ok := val.(*pay.Record)
	if !ok || m == nil {
		return false
	}
	return fik73RefForbidden(m.Ext, m.Ref)
}

// giro01ReferenceTooLong reports whether a Giro (payment-means 50) instruction
// using kortart 01 carries a payment reference longer than the 16 characters
// OIOUBL allows in cbc:InstructionID (F-LIB149).
func giro01ReferenceTooLong(val any) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil {
		return false
	}
	return instr.Ext.Get(untdid.ExtKeyPaymentMeans) == "50" &&
		instr.Ext.Get(ExtKeyPaymentID) == "01" &&
		len(instr.Ref.String()) > 16
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

// paymentIDInvalidFor reports whether the instruction uses the given OIOUBL
// payment-means code but lacks a dk-oioubl-payment-id from the allowed set
// (covering both the mandatory-presence and the codelist checks).
func paymentIDInvalidFor(val any, code cbc.Code, allowed []cbc.Code) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil {
		return false
	}
	if instr.Ext.Get(untdid.ExtKeyPaymentMeans) != code {
		return false
	}
	return !instr.Ext.Get(ExtKeyPaymentID).In(allowed...)
}

func roundingInRange(val any) bool {
	var a num.Amount
	switch v := val.(type) {
	case num.Amount:
		a = v
	case *num.Amount:
		if v == nil {
			return true
		}
		a = *v
	default:
		return true
	}
	return a.Compare(roundingMin) >= 0 && a.Compare(roundingMax) <= 0
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
