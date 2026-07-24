package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
)

// Outbound moves DueDate onto PaymentMeans[0].PaymentDueDate and blanks it;
// invoices (unlike credit notes) never fall back to reading it from there.
func (ui *Invoice) stripPaymentDueDate() {
	if ui.DueDate != "" || len(ui.PaymentMeans) == 0 || ui.PaymentMeans[0].PaymentDueDate == nil {
		return
	}
	ui.DueDate = *ui.PaymentMeans[0].PaymentDueDate
}

func decoratePayment(payment *bill.PaymentDetails, pm *ubl.PaymentMeans) {
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
		decorateGiroFIK(instr, pm)
	case paymentChannelDKBank:
		decorateDKBank(instr)
	case paymentChannelIBAN:
		decorateIBAN(instr, pm)
	}
}

// The real ref is under InstructionID (PaymentID carries a routing code
// instead); for FIK, the creditor account is under CreditAccount.
func decorateGiroFIK(instr *pay.Instructions, pm *ubl.PaymentMeans) {
	instr.Ref = ""
	if pm.InstructionID != nil {
		instr.Ref = cbc.Code(cleanString(*pm.InstructionID))
	}
	if pm.CreditAccount != nil && pm.CreditAccount.AccountID != "" {
		instr.CreditTransfer = []*pay.CreditTransfer{{Number: pm.CreditAccount.AccountID}}
	}
}

// What the base parser read as a BIC is really the bank reg. number (F-LIB124/130).
func decorateDKBank(instr *pay.Instructions) {
	if len(instr.CreditTransfer) == 0 || instr.CreditTransfer[0].BIC == "" {
		return
	}
	ct := instr.CreditTransfer[0]
	ct.Branch = &org.Address{Label: ct.BIC}
	ct.BIC = ""
}

// OIOUBL nests the BIC one level deeper than the base parser looks.
func decorateIBAN(instr *pay.Instructions, pm *ubl.PaymentMeans) {
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
