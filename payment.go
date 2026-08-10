package oioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/pay"
)

// OIOUBL paymentchannelcode-1.1 wire values derived from the payment means
// (see paymentChannel); the codelist's other values aren't supported outbound.
const (
	paymentChannelIBAN     = "IBAN"
	paymentChannelGiro     = "DK:GIRO"
	paymentChannelFIK      = "DK:FIK"
	paymentChannelDKBank   = "DK:BANK"
	paymentChannelNemKonto = "DK:NEMKONTO"
)

// applyPayment adjusts the base's already-built PaymentMeans for OIOUBL:
// stamps the channel code, and replaces the Giro/FIK shape outright.
func (ui *Invoice) applyPayment(inv *bill.Invoice) {
	if inv == nil || inv.Payment == nil || inv.Payment.Instructions == nil || len(ui.PaymentMeans) == 0 {
		applyPaymentTermsAmount(ui)
		return
	}
	instr := inv.Payment.Instructions
	paymentMeansCode := instr.Ext.Get(untdid.ExtKeyPaymentMeans).String()
	pm := &ui.PaymentMeans[0]

	switch paymentMeansCode {
	case "50", "93":
		applyPaymentID(pm, instr, paymentMeansCode)
		if paymentMeansCode == "93" && len(instr.CreditTransfer) > 0 {
			// FIK: cac:CreditAccount/cbc:AccountID (F-LIB305), not PayeeFinancialAccount.
			pm.CreditAccount = &ubl.CreditAccount{AccountID: instr.CreditTransfer[0].Number.String()}
			pm.PayeeFinancialAccount = nil
		}
	case "42":
		applyRegNr(pm, instr)
	}
	if ch := paymentChannel(paymentMeansCode); ch != "" {
		pm.PaymentChannelCode = &ubl.IDType{Value: ch}
	}

	applyPaymentMeans(ui)
	applyPaymentTermsAmount(ui)
}

// applyPaymentTermsAmount stamps F-INV134: the payment terms carry the payable amount in OIOUBL.
func applyPaymentTermsAmount(ui *Invoice) {
	if ui.PaymentTerms != nil && ui.PaymentTerms.Amount == nil && ui.LegalMonetaryTotal.PayableAmount != nil {
		ui.PaymentTerms.Amount = &ubl.Amount{
			Value:      ui.LegalMonetaryTotal.PayableAmount.Value,
			CurrencyID: ui.LegalMonetaryTotal.PayableAmount.CurrencyID,
		}
	}
}

func paymentChannel(means string) string {
	switch means {
	case "50":
		return paymentChannelGiro
	case "93":
		return paymentChannelFIK
	case "42":
		return paymentChannelDKBank
	case "97":
		return paymentChannelNemKonto
	case "30", "31", "58":
		return paymentChannelIBAN
	}
	return ""
}

// applyRegNr moves the reg. nr. from the credit transfer's name, where EN 16931
// has to keep it, to the branch where OIOUBL wants it (F-LIB124/F-LIB130).
func applyRegNr(pm *ubl.PaymentMeans, instr *pay.Instructions) {
	if pm.PayeeFinancialAccount == nil {
		return
	}
	pm.PayeeFinancialAccount.FinancialInstitutionBranch = nil
	if len(instr.CreditTransfer) == 0 {
		return
	}
	regNr := instr.CreditTransfer[0].Name
	if regNr == "" {
		return
	}
	pm.PayeeFinancialAccount.Name = nil
	pm.PayeeFinancialAccount.FinancialInstitutionBranch = &ubl.Branch{ID: &regNr}
}

// applyPaymentMeans stamps the payment channel and moves the document due date onto each means.
func applyPaymentMeans(out *Invoice) {
	for i := range out.PaymentMeans {
		pm := &out.PaymentMeans[i]
		stampPaymentChannel(pm)
		if out.DueDate != "" && pm.PaymentDueDate == nil {
			pm.PaymentDueDate = ptr(out.DueDate)
		}
	}
	if len(out.PaymentMeans) > 0 && out.DueDate != "" {
		out.DueDate = ""
	}
}

// stampPaymentChannel stamps the paymentchannelcode-1.1 list ID and, for IBAN
// accounts, nests the base's branch ID (the BIC) under FinancialInstitution
// and drops the redundant branch ID itself (F-LIB295).
func stampPaymentChannel(pm *ubl.PaymentMeans) {
	if pm.PaymentChannelCode == nil {
		return
	}
	pm.PaymentChannelCode.ListID = ptr(codelistPaymentChannel)
	if pm.PaymentChannelCode.Value != paymentChannelIBAN || pm.PayeeFinancialAccount == nil {
		return
	}
	branch := pm.PayeeFinancialAccount.FinancialInstitutionBranch
	if branch == nil || branch.ID == nil {
		return
	}
	branch.FinancialInstitution = &ubl.FinancialInstitution{ID: branch.ID}
	branch.ID = nil
}

// kortart derives the Giro/FIK "kortart" (cbc:PaymentID) from the payment means
// and reference; malformed values are left for the schematron to reject.
func kortart(paymentMeansCode, ref string) string {
	switch paymentMeansCode {
	case "93":
		switch {
		case ref == "":
			return "73"
		case len(ref) == 16:
			return "75"
		default:
			return "71"
		}
	case "50":
		if ref == "" {
			return "01"
		}
		return "04"
	}
	return ""
}

// applyPaymentID sets the Giro (50) / FIK (93) cbc:PaymentID kortart; the
// reference rides cbc:InstructionID for structured kortarts (not free-text 01/73).
func applyPaymentID(pm *ubl.PaymentMeans, instr *pay.Instructions, paymentMeansCode string) {
	if paymentMeansCode != "50" && paymentMeansCode != "93" {
		return
	}
	ref := instr.Ref.String()
	k := kortart(paymentMeansCode, ref)
	pm.PaymentID = ptr(k)
	pm.InstructionID = nil
	if ref != "" && k != "01" && k != "73" {
		pm.InstructionID = ptr(ref)
	}
}
