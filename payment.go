package dkoioubl

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/pay"
)

// decoratePayment adjusts the base's already-built PaymentMeans/PaymentTerms
// (from gobl.ubl's AddPayment, which ran as part of the base ConvertInvoice
// call — including the BT-90 creditor identifier and the ordinary bank
// transfer/direct debit/card case) for OIOUBL: stamps the payment channel
// code, and replaces the Giro/FIK kortart and account shape outright, since
// those have no equivalent in the generic base (cac:CreditAccount, instead of
// cac:PayeeFinancialAccount, plus the "kortart" cbc:PaymentID).
func (ui *Invoice) decoratePayment(inv *bill.Invoice) error {
	if inv == nil || inv.Payment == nil || inv.Payment.Instructions == nil || len(ui.PaymentMeans) == 0 {
		applyPaymentTermsAmount(ui)
		return nil
	}
	instr := inv.Payment.Instructions
	paymentMeansCode := instr.Ext.Get(untdid.ExtKeyPaymentMeans).String()
	pm := &ui.PaymentMeans[0]

	switch paymentMeansCode {
	case "50", "93":
		applyPaymentID(pm, instr, paymentMeansCode)
		if paymentMeansCode == "93" && len(instr.CreditTransfer) > 0 {
			// FIK: cac:CreditAccount/cbc:AccountID (F-LIB305), not PayeeFinancialAccount.
			pm.CreditAccount = &CreditAccount{AccountID: instr.CreditTransfer[0].Number}
			pm.PayeeFinancialAccount = nil
		}
	case "42":
		applyRegNr(pm, instr)
	case "97":
		clearNemKontoDetails(pm)
	}
	if ch := paymentChannel(paymentMeansCode); ch != "" {
		pm.PaymentChannelCode = &IDType{Value: ch}
	}

	applyPaymentMeans(ui)
	applyPaymentTermsAmount(ui)
	return nil
}

// applyPaymentTermsAmount stamps F-INV134: the payment terms carry the payable amount in OIOUBL.
func applyPaymentTermsAmount(ui *Invoice) {
	if ui.PaymentTerms != nil && ui.PaymentTerms.Amount == nil && ui.LegalMonetaryTotal.PayableAmount != nil {
		ui.PaymentTerms.Amount = &Amount{
			Value:      ui.LegalMonetaryTotal.PayableAmount.Value,
			CurrencyID: ui.LegalMonetaryTotal.PayableAmount.CurrencyID,
		}
	}
}

// OIOUBL paymentchannelcode-1.1 wire values derived from the payment means
// (see paymentChannel); the codelist's other values aren't supported outbound.
const (
	paymentChannelIBAN     = "IBAN"
	paymentChannelGiro     = "DK:GIRO"
	paymentChannelFIK      = "DK:FIK"
	paymentChannelDKBank   = "DK:BANK"
	paymentChannelNemKonto = "DK:NEMKONTO"
)

// paymentChannel maps a UNTDID 4461 payment means to its OIOUBL
// paymentchannelcode-1.1 value: Giro (50) → DK:GIRO, FIK (93) → DK:FIK,
// domestic bank transfers (42) → DK:BANK, NemKonto (97) → DK:NEMKONTO,
// account transfers (30/31/58) → IBAN. Every other means carries none.
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

// applyRegNr replaces the branch the base built from the BIC: for a domestic
// bank transfer (42) the flat FinancialInstitutionBranch/ID is the Danish bank
// registration number — 4 digits, carried on the credit transfer's branch
// label — and never a BIC (F-LIB124 / F-LIB130).
func applyRegNr(pm *PaymentMeans, instr *pay.Instructions) {
	if pm.PayeeFinancialAccount == nil {
		return
	}
	pm.PayeeFinancialAccount.FinancialInstitutionBranch = nil
	if len(instr.CreditTransfer) == 0 {
		return
	}
	branch := instr.CreditTransfer[0].Branch
	if branch == nil || branch.Label == "" {
		return
	}
	regNr := branch.Label
	pm.PayeeFinancialAccount.FinancialInstitutionBranch = &Branch{ID: &regNr}
}

// clearNemKontoDetails strips everything but the means and channel codes: a
// NemKonto payment (97) is disbursed to the payee's centrally registered
// account, resolved by the payer out-of-band, so the means allows no account
// or payment identification at all (F-LIB159 – F-LIB165).
func clearNemKontoDetails(pm *PaymentMeans) {
	pm.InstructionID = nil
	pm.InstructionNote = nil
	pm.PaymentID = nil
	pm.CardAccount = nil
	pm.PayerFinancialAccount = nil
	pm.PayeeFinancialAccount = nil
	pm.CreditAccount = nil
	pm.PaymentMandate = nil
}

// applyPaymentMeans stamps the payment channel and moves the document due date onto each means.
func applyPaymentMeans(out *Invoice) {
	for i := range out.PaymentMeans {
		pm := &out.PaymentMeans[i]
		stampPaymentChannel(pm)
		if out.DueDate != "" && pm.PaymentDueDate == nil {
			d := out.DueDate
			pm.PaymentDueDate = &d
		}
	}
	if len(out.PaymentMeans) > 0 && out.DueDate != "" {
		out.DueDate = ""
	}
}

// stampPaymentChannel stamps the paymentchannelcode-1.1 list ID and, for IBAN
// accounts, nests the base's branch ID (the BIC) under FinancialInstitution
// and drops the redundant branch ID itself (F-LIB295).
func stampPaymentChannel(pm *PaymentMeans) {
	if pm.PaymentChannelCode == nil {
		return
	}
	listID := listPaymentChannel
	pm.PaymentChannelCode.ListID = &listID
	if pm.PaymentChannelCode.Value != paymentChannelIBAN || pm.PayeeFinancialAccount == nil {
		return
	}
	branch := pm.PayeeFinancialAccount.FinancialInstitutionBranch
	if branch == nil || branch.ID == nil {
		return
	}
	branch.FinancialInstitution = &FinancialInstitution{ID: branch.ID}
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
func applyPaymentID(pm *PaymentMeans, instr *pay.Instructions, paymentMeansCode string) {
	if paymentMeansCode != "50" && paymentMeansCode != "93" {
		return
	}
	ref := instr.Ref.String()
	k := kortart(paymentMeansCode, ref)
	pm.PaymentID = &k
	pm.InstructionID = nil
	if ref != "" && k != "01" && k != "73" {
		pm.InstructionID = &ref
	}
}
