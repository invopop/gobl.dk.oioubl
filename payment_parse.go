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
	if payment == nil || payment.Instructions == nil {
		return
	}
	instr := payment.Instructions
	instr.Key = pay.MeansKeyOther
	// The base parse's own Calculate already re-derived Ext from Key once
	// (e.g. UBL "42" -> MeansKeyDebitTransfer -> en16931's canonical "31"); reassert the real wire code.
	instr.Ext = instr.Ext.Set(untdid.ExtKeyPaymentMeans, cbc.Code(pm.PaymentMeansCode.Value))

	// The channel is optional, and OIOUBL's own samples omit it; the wire means
	// code above is worth keeping either way.
	if pm.PaymentChannelCode == nil {
		return
	}

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
		instr.CreditTransfer = []*pay.CreditTransfer{{Number: cbc.Code(pm.CreditAccount.AccountID)}}
	}
}

// fixDKBank moves the bank registration number off the BIC, where the base
// parser puts whatever the branch carries. On a Danish domestic transfer that
// element is the reg. nr. (F-LIB124/130), which belongs in the clearing code.
func fixDKBank(instr *pay.Instructions) {
	if len(instr.CreditTransfer) == 0 || instr.CreditTransfer[0].BIC.IsEmpty() {
		return
	}
	ct := instr.CreditTransfer[0]
	ct.Clearing = ct.BIC
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
	instr.CreditTransfer[0].BIC = cbc.Code(cleanString(*branch.FinancialInstitution.ID))
}
