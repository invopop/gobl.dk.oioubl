package dkoioubl

import (
	"errors"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/validation"
)

func (ui *Invoice) addPayment(inv *bill.Invoice) error {
	if inv == nil || inv.Payment == nil {
		return nil
	}
	pymt := inv.Payment

	if pymt.Instructions != nil {
		if err := ui.addPaymentInstructions(inv); err != nil {
			return err
		}
	}

	if pymt.Terms != nil {
		ui.addPaymentTerms(pymt)
	}

	if pymt.Payee != nil {
		ui.PayeeParty = newPayeeParty(pymt.Payee)
	}

	// BT-90: creditor identifier, carried as a SEPA PartyIdentification on the payee (or seller).
	if pymt.Instructions != nil && pymt.Instructions.DirectDebit != nil && pymt.Instructions.DirectDebit.Creditor != "" {
		sepaID := "SEPA"
		id := Identification{
			ID: &IDType{
				Value:    pymt.Instructions.DirectDebit.Creditor,
				SchemeID: &sepaID,
			},
		}
		if ui.PayeeParty != nil {
			ui.PayeeParty.PartyIdentification = append(ui.PayeeParty.PartyIdentification, id)
		} else {
			ui.AccountingSupplierParty.Party.PartyIdentification = append(ui.AccountingSupplierParty.Party.PartyIdentification, id)
		}
	}

	applyPaymentMeans(ui)
	// F-INV134: the payment terms carry the payable amount in OIOUBL.
	if ui.PaymentTerms != nil && ui.PaymentTerms.Amount == nil && ui.LegalMonetaryTotal.PayableAmount != nil {
		ui.PaymentTerms.Amount = &Amount{
			Value:      ui.LegalMonetaryTotal.PayableAmount.Value,
			CurrencyID: ui.LegalMonetaryTotal.PayableAmount.CurrencyID,
		}
	}

	return nil
}

// OIOUBL paymentchannelcode-1.1 wire values, derived from the payment means (see paymentChannel).
const (
	paymentChannelIBAN = "IBAN"
	paymentChannelGiro = "DK:GIRO"
	paymentChannelFIK  = "DK:FIK"
)

// paymentChannel maps a UNTDID 4461 payment means to its OIOUBL
// paymentchannelcode-1.1 value: Giro (50) → DK:GIRO, FIK (93) → DK:FIK,
// account transfers (30/31/58) → IBAN. Every other means carries none.
func paymentChannel(means string) string {
	switch means {
	case "50":
		return paymentChannelGiro
	case "93":
		return paymentChannelFIK
	case "30", "31", "58":
		return paymentChannelIBAN
	}
	return ""
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

// stampPaymentChannel stamps the paymentchannelcode-1.1 list ID and strips the
// redundant branch ID from IBAN accounts (F-LIB295; the BIC stays under FinancialInstitution/ID).
func stampPaymentChannel(pm *PaymentMeans) {
	if pm.PaymentChannelCode == nil {
		return
	}
	listID := listPaymentChannel
	pm.PaymentChannelCode.ListID = &listID
	if pm.PaymentChannelCode.Value == paymentChannelIBAN && pm.PayeeFinancialAccount != nil && pm.PayeeFinancialAccount.FinancialInstitutionBranch != nil {
		pm.PayeeFinancialAccount.FinancialInstitutionBranch.ID = nil
	}
}

func (ui *Invoice) addPaymentInstructions(inv *bill.Invoice) error {
	instr := inv.Payment.Instructions
	if instr.Ext.IsZero() || instr.Ext.Get(untdid.ExtKeyPaymentMeans).String() == "" {
		return validation.Errors{
			"instructions": validation.Errors{
				extFieldKey: validation.Errors{
					untdid.ExtKeyPaymentMeans.String(): errors.New("required"),
				},
			},
		}
	}
	paymentMeansCode := instr.Ext.Get(untdid.ExtKeyPaymentMeans).String()
	ui.PaymentMeans = []PaymentMeans{
		{
			PaymentMeansCode: IDType{Value: paymentMeansCode},
		},
	}
	if instr.Meta != nil {
		if channel, ok := instr.Meta[cbc.Key("payment-channel")]; ok && channel != "" {
			ui.PaymentMeans[0].PaymentChannelCode = &IDType{Value: channel}
		}
	}
	if ref := instr.Ref.String(); ref != "" {
		ui.PaymentMeans[0].PaymentID = &ref
	}
	// Payment channel and the Giro/FIK kortart both derive from the means (see applyPaymentID).
	if ch := paymentChannel(paymentMeansCode); ch != "" && ui.PaymentMeans[0].PaymentChannelCode == nil {
		ui.PaymentMeans[0].PaymentChannelCode = &IDType{Value: ch}
	}
	applyPaymentID(&ui.PaymentMeans[0], instr, paymentMeansCode)
	if instr.Detail != "" {
		ui.PaymentMeans[0].PaymentMeansCode.Name = &instr.Detail
	}
	ui.addCreditTransferAccount(instr, paymentMeansCode)
	if instr.DirectDebit != nil {
		// Skip the mandate without a reference; an empty <cbc:ID/> is rejected downstream.
		if instr.DirectDebit.Ref != "" {
			ui.PaymentMeans[0].PaymentMandate = &PaymentMandate{
				ID: &IDType{Value: instr.DirectDebit.Ref},
			}
		}
		if instr.DirectDebit.Account != "" {
			ui.PaymentMeans[0].PayerFinancialAccount = &FinancialAccount{
				ID: &instr.DirectDebit.Account,
			}
		}
	}
	if instr.Card != nil {
		ui.PaymentMeans[0].CardAccount = &CardAccount{
			PrimaryAccountNumberID: &instr.Card.Last4,
		}
		if instr.Card.Holder != "" {
			ui.PaymentMeans[0].CardAccount.HolderName = &instr.Card.Holder
		}
	}
	if ui.CreditNoteTypeCode != nil && inv.Payment.Terms != nil && len(inv.Payment.Terms.DueDates) > 0 {
		formattedDate := ubl.FormatDate(*inv.Payment.Terms.DueDates[0].Date)
		ui.PaymentMeans[0].PaymentDueDate = &formattedDate
	}
	return nil
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
	if ref != "" && k != "01" && k != "73" {
		pm.InstructionID = &ref
	}
}

// addCreditTransferAccount wires the credit-transfer account onto the payment
// means. FIK (93) uses cac:CreditAccount/cbc:AccountID (F-LIB305), not PayeeFinancialAccount.
func (ui *Invoice) addCreditTransferAccount(instr *pay.Instructions, paymentMeansCode string) {
	if len(instr.CreditTransfer) == 0 {
		return
	}
	pm := &ui.PaymentMeans[0]
	if paymentMeansCode == "93" {
		pm.CreditAccount = &CreditAccount{AccountID: instr.CreditTransfer[0].Number}
		return
	}
	pm.PayeeFinancialAccount = newCreditTransferAccount(instr.CreditTransfer[0], paymentMeansCode)
}

// Adapted from gobl.ubl; OIOUBL: nests the BIC under FinancialInstitution for IBAN channels 31/58 (F-LIB295).
func newCreditTransferAccount(ct *pay.CreditTransfer, paymentMeansCode string) *FinancialAccount {
	pfa := new(FinancialAccount)
	if ct.IBAN != "" {
		pfa.ID = &ct.IBAN
	} else if ct.Number != "" {
		pfa.ID = &ct.Number
	}
	if ct.Name != "" {
		pfa.Name = &ct.Name
	}
	if ct.BIC != "" {
		branch := &Branch{ID: &ct.BIC}
		// IBAN-channel transfers (31, 58) nest the BIC under FinancialInstitution;
		// the branch ID is then stripped for the IBAN channel (F-LIB295), which
		// would otherwise lose the BIC.
		if paymentMeansCode == "31" || paymentMeansCode == "58" {
			branch.FinancialInstitution = &FinancialInstitution{
				ID: &ct.BIC,
			}
		}
		pfa.FinancialInstitutionBranch = branch
	}
	return pfa
}

func (ui *Invoice) addPaymentTerms(pymt *bill.PaymentDetails) {
	if pymt.Terms.Notes != "" {
		ui.PaymentTerms = &PaymentTerms{
			Note: pymt.Terms.Notes,
		}
	}

	// Only one due date allowed under EN 16931
	if ui.CreditNoteTypeCode == nil && len(pymt.Terms.DueDates) > 0 && pymt.Terms.DueDates[0].Date != nil {
		ui.DueDate = ubl.FormatDate(*pymt.Terms.DueDates[0].Date)
	}
}
