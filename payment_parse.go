package oioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/pay"
)

// stripPaymentDueDate puts the due date back where the generic parser expects
// it, since on an invoice it never looks under the payment means.
func (ui *Invoice) stripPaymentDueDate() {
	if ui.DueDate != "" || len(ui.PaymentMeans) == 0 || ui.PaymentMeans[0].PaymentDueDate == nil {
		return
	}
	ui.DueDate = *ui.PaymentMeans[0].PaymentDueDate
}

func addPaymentDetails(payment *bill.PaymentDetails, pm *ubl.PaymentMeans) {
	if payment == nil || payment.Instructions == nil || pm.PaymentChannelCode == nil {
		return
	}
	instr := payment.Instructions
	instr.Key = pay.MeansKeyOther
	// The base parse's own Calculate already re-derived Ext from Key once
	// (e.g. UBL "42" -> MeansKeyDebitTransfer -> en16931's canonical "31"); reassert the real wire code.
	instr.Ext = instr.Ext.Set(untdid.ExtKeyPaymentMeans, cbc.Code(pm.PaymentMeansCode.Value))

	switch pm.PaymentChannelCode.Value {
	case paymentChannelGiro, paymentChannelFIK:
		fixGiroFIK(instr, pm)
	case paymentChannelDKBank:
		fixDKBank(instr)
	case paymentChannelIBAN:
		fixIBAN(instr, pm)
	}
}

// fixGiroFIK reads the payment reference from InstructionID; PaymentID holds a
// routing code, not the reference.
func fixGiroFIK(instr *pay.Instructions, pm *ubl.PaymentMeans) {
	instr.Ref = ""
	if pm.InstructionID != nil {
		instr.Ref = cbc.Code(cleanString(*pm.InstructionID))
	}
	if pm.CreditAccount != nil && pm.CreditAccount.AccountID != "" {
		instr.CreditTransfer = []*pay.CreditTransfer{{Number: pm.CreditAccount.AccountID}}
	}
}

// fixDKBank corrects the bank registration number, which the base parser takes
// for a BIC (F-LIB124/130). GOBL has one name to hold it, so a document that
// also names the account loses that name: the reg. nr. is what a domestic
// transfer cannot go out without.
func fixDKBank(instr *pay.Instructions) {
	if len(instr.CreditTransfer) == 0 || instr.CreditTransfer[0].BIC == "" {
		return
	}
	ct := instr.CreditTransfer[0]
	ct.Name = ct.BIC
	ct.BIC = ""
}

// fixIBAN finds the BIC, which OIOUBL nests one level deeper than the base
// parser looks.
func fixIBAN(instr *pay.Instructions, pm *ubl.PaymentMeans) {
	if len(instr.CreditTransfer) == 0 || instr.CreditTransfer[0].BIC != "" {
		return
	}
	account := pm.PayeeFinancialAccount
	if account == nil || account.FinancialInstitutionBranch == nil {
		return
	}
	branch := account.FinancialInstitutionBranch
	if branch.FinancialInstitution == nil || branch.FinancialInstitution.ID == nil {
		return
	}
	instr.CreditTransfer[0].BIC = cleanString(*branch.FinancialInstitution.ID)
}
