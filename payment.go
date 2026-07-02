package dkoioubl

import (
	"errors"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/validation"
)

// PaymentMeans represents the means of payment
type PaymentMeans struct {
	PaymentMeansCode      IDType            `xml:"cbc:PaymentMeansCode"`
	PaymentDueDate        *string           `xml:"cbc:PaymentDueDate,omitempty"`
	PaymentChannelCode    *IDType           `xml:"cbc:PaymentChannelCode,omitempty"`
	InstructionID         *string           `xml:"cbc:InstructionID"`
	InstructionNote       []string          `xml:"cbc:InstructionNote,omitempty"`
	PaymentID             *string           `xml:"cbc:PaymentID"`
	CardAccount           *CardAccount      `xml:"cac:CardAccount"`
	PayerFinancialAccount *FinancialAccount `xml:"cac:PayerFinancialAccount"`
	PayeeFinancialAccount *FinancialAccount `xml:"cac:PayeeFinancialAccount"`
	CreditAccount         *CreditAccount    `xml:"cac:CreditAccount"`
	PaymentMandate        *PaymentMandate   `xml:"cac:PaymentMandate"`
}

// CreditAccount carries the OIOUBL FIK creditor account (cbc:AccountID).
type CreditAccount struct {
	AccountID string `xml:"cbc:AccountID"`
}

// PaymentMandate represents a payment mandate
type PaymentMandate struct {
	ID                    IDType            `xml:"cbc:ID"`
	PayerFinancialAccount *FinancialAccount `xml:"cac:PayerFinancialAccount"`
}

// CardAccount represents a card account
type CardAccount struct {
	PrimaryAccountNumberID *string `xml:"cbc:PrimaryAccountNumberID"`
	NetworkID              *string `xml:"cbc:NetworkID"`
	HolderName             *string `xml:"cbc:HolderName"`
}

// FinancialAccount represents a financial account
type FinancialAccount struct {
	ID                         *string `xml:"cbc:ID"`
	Name                       *string `xml:"cbc:Name"`
	FinancialInstitutionBranch *Branch `xml:"cac:FinancialInstitutionBranch"`
	AccountTypeCode            *string `xml:"cbc:AccountTypeCode"`
}

// Branch represents a branch of a financial institution
type Branch struct {
	ID                   *string               `xml:"cbc:ID"`
	Name                 *string               `xml:"cbc:Name"`
	FinancialInstitution *FinancialInstitution `xml:"cac:FinancialInstitution"`
}

// FinancialInstitution represents a financial institution.
type FinancialInstitution struct {
	ID *string `xml:"cbc:ID"`
}

// PaymentTerms represents the terms of payment
type PaymentTerms struct {
	Note   string  `xml:"cbc:Note,omitempty"`
	Amount *Amount `xml:"cbc:Amount,omitempty"`
}

// PrepaidPayment represents a prepaid payment
type PrepaidPayment struct {
	ID            string  `xml:"cbc:ID"`
	PaidAmount    *Amount `xml:"cbc:PaidAmount"`
	ReceivedDate  *string `xml:"cbc:ReceivedDate"`
	InstructionID *string `xml:"cbc:InstructionID"`
}

const sepaSchemeID = "SEPA"

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

	// BT-90: Bank assigned creditor identifier
	// In UBL this lives as a SEPA PartyIdentification on the payee (or seller)
	if pymt.Instructions != nil && pymt.Instructions.DirectDebit != nil && pymt.Instructions.DirectDebit.Creditor != "" {
		sepaID := sepaSchemeID
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

// OIOUBL paymentchannelcode-1.1 wire values. Derived from the payment means
// (see paymentChannel), not carried in an extension.
const (
	paymentChannelIBAN = "IBAN"
	paymentChannelGiro = "DK:GIRO"
	paymentChannelFIK  = "DK:FIK"
)

// paymentChannel maps a UNTDID 4461 payment means to its OIOUBL
// paymentchannelcode-1.1 value: Giro (50) → DK:GIRO, FIK (93) → DK:FIK, and the
// account-transfer means (30/31 bank transfer, 58 SEPA credit transfer) → IBAN.
// Every other means settles outside a payment channel and carries none.
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

// applyPaymentMeans stamps the payment channel (see stampPaymentChannel) and
// moves the document due date onto each means.
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
// redundant FinancialInstitutionBranch from IBAN accounts (F-LIB295, the BIC
// stays nested under FinancialInstitution/ID). The channel value itself is set
// when the payment means is built.
func stampPaymentChannel(pm *PaymentMeans) {
	if pm.PaymentChannelCode == nil {
		return
	}
	pm.PaymentChannelCode.ListID = ptr(listPaymentChannel)
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
	// The payment channel and the Giro/FIK PaymentID kortart are both derived
	// from the payment means and reference (see applyPaymentID).
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
				ID: IDType{Value: instr.DirectDebit.Ref},
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
		formattedDate := formatDate(*inv.Payment.Terms.DueDates[0].Date)
		ui.PaymentMeans[0].PaymentDueDate = &formattedDate
	}
	return nil
}

// kortart deduces the Giro/FIK "kortart" (cbc:PaymentID) from the payment
// reference. FIK (means 93): no reference is the free-text kortart 73, a
// 16-character reference is 75, and any other reference is 71 (whose 15-character
// length F-LIB156 then confirms). Giro (means 50): the free-text kortart 01
// without a reference, 04 (creditor number) with one — the two-reference form 15
// isn't distinguishable from GOBL data. A malformed reference is left for the
// schematron to reject (F-LIB149/156/157/312/336).
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

// applyPaymentID sets the Giro (50) / FIK (93) cbc:PaymentID to the kortart
// deduced for the payment reference; the reference itself rides cbc:InstructionID
// for the structured kortarts (the free-text 01/73 carry no payment number).
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
// means. For FIK (93) the creditor account lives in cac:CreditAccount/cbc:AccountID
// (8 chars, F-LIB305) rather than PayeeFinancialAccount.
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
		// IBAN-channel transfers (domestic 31, SEPA 58) nest the BIC under
		// FinancialInstitution; the redundant branch ID is then stripped for the
		// IBAN channel (F-LIB295), so without the nesting the BIC would be lost
		// and the branch left empty.
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
		ui.DueDate = formatDate(*pymt.Terms.DueDates[0].Date)
	}
}
