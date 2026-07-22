package addon

import (
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// validPaymentMeansCodes are the UNTDID 4461 means accepted for OIOUBL (F-LIB100).
var validPaymentMeansCodes = []cbc.Code{
	"1", "10", "20", "31", "42", "48", "49", "50", "58", "59", "93", "97",
}

// bankTransferCodes are the OIOUBL PaymentMeansCode values requiring a payee
// account (F-LIB107 for 31, F-LIB377 for 58).
var bankTransferCodes = []cbc.Code{"31", "58"}

// normalizePayInstructions stamps DK on a domestic bank transfer's (means 42)
// credit-transfer branch, since only the reg. nr. is set there and EN 16931
// requires a country on every address (BR-9 et al.).
func normalizePayInstructions(instr *pay.Instructions) {
	if instr.Ext.Get(untdid.ExtKeyPaymentMeans) != "42" {
		return
	}
	ct := firstCreditTransfer(instr)
	if ct == nil || ct.Branch == nil || ct.Branch.Country != "" {
		return
	}
	ct.Branch.Country = "DK"
}

func payInstructionsRules() *rules.Set {
	return rules.For(new(pay.Instructions),
		rules.Field("ext",
			rules.AssertIfPresent("01", "payment-means code must be one of the OIOUBL allowed values (F-LIB100)",
				tax.ExtensionsHasCodes(untdid.ExtKeyPaymentMeans, validPaymentMeansCodes...)),
		),
		rules.Assert("02", "a credit transfer account (IBAN or number) is required for bank-transfer payment means (F-LIB107 / F-LIB377)",
			is.Func("bank-transfer has a payee account", bankTransferHasAccount)),
		rules.Assert("03", "a BIC is required on the credit transfer for IBAN bank-transfer payment means 31 (F-LIB113)",
			is.Func("iban bank-transfer has a BIC", ibanTransferHasBIC)),
		rules.Assert("04", "Giro (payment-means 50) requires a 7 or 8 digit payee account (F-LIB319 / F-LIB320 / F-LIB321)",
			is.Func("giro has a 7-8 digit payee account", giroAccountValid)),
		rules.Assert("05", "FIK (payment-means 93) requires an 8-character creditor account (F-LIB305)",
			is.Func("fik has an 8-character creditor account", fikAccountValid)),
		rules.Assert("06", "a domestic bank transfer (payment-means 42) requires a payee account number of at most 10 characters (F-LIB126 / F-LIB131)",
			is.Func("dk bank transfer has a valid account number", dkBankAccountValid)),
		rules.Assert("07", "a domestic bank transfer (payment-means 42) requires the bank registration number (up to 4 digits) as the credit-transfer branch label (F-LIB124 / F-LIB130)",
			is.Func("dk bank transfer has a bank registration number", dkBankRegNrValid)),
		rules.Assert("08", "NemKonto (payment-means 97) must not carry a credit transfer: the payer resolves the payee's registered account via NemKonto (F-LIB164)",
			is.Func("nemkonto has no credit transfer", nemKontoHasNoCreditTransfer)),
	)
}

// billPayTermsRules relaxes EN 16931 BR-CO-25: OIOUBL allows bare invoice payment
// terms (ID + amount only), so the due-dates-or-notes requirement doesn't apply.
func billPayTermsRules() *rules.Set {
	return rules.For(new(pay.Terms),
		rules.Ignore("GOBL-EU-EN16931-PAY-TERMS-01"),
	)
}

func ibanTransferHasBIC(val any) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil {
		return true
	}
	if instr.Ext.Get(untdid.ExtKeyPaymentMeans) != "31" {
		return true
	}
	ct := firstCreditTransfer(instr)
	// A missing account is rule 02's concern (F-LIB113 covers only the BIC).
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

// dkBankAccountValid checks F-LIB126/F-LIB131: a domestic bank transfer
// (means 42) must carry a payee account number of at most 10 characters.
func dkBankAccountValid(val any) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil || instr.Ext.Get(untdid.ExtKeyPaymentMeans) != "42" {
		return true
	}
	ct := firstCreditTransfer(instr)
	return ct != nil && ct.Number != "" && len(ct.Number) <= 10
}

// dkBankRegNrValid checks the bank registration number (1-4 digits) on the
// credit-transfer branch label (F-LIB124/F-LIB130).
func dkBankRegNrValid(val any) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil || instr.Ext.Get(untdid.ExtKeyPaymentMeans) != "42" {
		return true
	}
	ct := firstCreditTransfer(instr)
	return ct != nil && ct.Branch != nil && isNumericOfLen(ct.Branch.Label, 1, 4)
}

// nemKontoHasNoCreditTransfer checks that a NemKonto payment carries no
// credit transfer, since the payer looks up the account itself (F-LIB164).
func nemKontoHasNoCreditTransfer(val any) bool {
	instr, ok := val.(*pay.Instructions)
	if !ok || instr == nil || instr.Ext.Get(untdid.ExtKeyPaymentMeans) != "97" {
		return true
	}
	return len(instr.CreditTransfer) == 0
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
