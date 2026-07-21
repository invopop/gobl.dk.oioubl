package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
)

func (ui *Invoice) goblAddPayment(out *bill.Invoice) error {
	payment := &bill.PaymentDetails{}

	if ui.PayeeParty != nil {
		payment.Payee = goblParty(ui.PayeeParty)
	}
	if err := ui.goblPaymentTerms(payment); err != nil {
		return err
	}
	if len(ui.PaymentMeans) > 0 {
		payment.Instructions = goblInvoiceInstructions(out, &ui.PaymentMeans[0])
	}
	if err := ui.goblPaymentAdvances(payment); err != nil {
		return err
	}

	if payment.Payee != nil || payment.Terms != nil || payment.Instructions != nil || len(payment.Advances) > 0 {
		out.Payment = payment
	}
	return nil
}

// goblPaymentTerms reads the due date (root, or the payment means when the
// root is absent, as on credit notes) plus any notes; a single due date takes 100%.
func (ui *Invoice) goblPaymentTerms(payment *bill.PaymentDetails) error {
	if ui.PaymentTerms != nil {
		payment.Terms = &pay.Terms{Notes: ubl.CleanString(ui.PaymentTerms.Note)}
	}

	var dueDate string
	if ui.CreditNoteTypeCode == nil {
		dueDate = ui.DueDate
	}
	if dueDate == "" && len(ui.PaymentMeans) > 0 && ui.PaymentMeans[0].PaymentDueDate != nil {
		dueDate = *ui.PaymentMeans[0].PaymentDueDate
	}
	if dueDate == "" {
		return nil
	}

	d, err := ubl.ParseDate(dueDate)
	if err != nil {
		return err
	}
	if payment.Terms == nil {
		payment.Terms = &pay.Terms{}
	}
	payment.Terms.DueDates = append(payment.Terms.DueDates, &pay.DueDate{Date: &d})

	if len(payment.Terms.DueDates) == 1 {
		percent, err := num.PercentageFromString("100%")
		if err != nil {
			return err
		}
		payment.Terms.DueDates[0].Percent = &percent
	}
	return nil
}

// goblPaymentAdvances reconstructs each cac:PrepaidPayment (F-INV131), or
// recovers a single advance from a total-only PrepaidAmount.
func (ui *Invoice) goblPaymentAdvances(payment *bill.PaymentDetails) error {
	switch {
	case len(ui.PrepaidPayment) > 0:
		payment.Advances = make([]*pay.Record, 0, len(ui.PrepaidPayment))
		for _, p := range ui.PrepaidPayment {
			if p.PaidAmount == nil {
				continue
			}
			amount, err := num.AmountFromString(ubl.NormalizeNumericString(p.PaidAmount.Value))
			if err != nil {
				return err
			}
			advance := &pay.Record{Amount: amount}
			if p.ReceivedDate != nil {
				d, err := ubl.ParseDate(*p.ReceivedDate)
				if err != nil {
					return err
				}
				advance.Date = &d
			}
			if p.InstructionID != nil {
				advance.Ref = *p.InstructionID
			}
			payment.Advances = append(payment.Advances, advance)
		}
	case ui.LegalMonetaryTotal.PrepaidAmount != nil:
		totalPrepaid, err := num.AmountFromString(ubl.NormalizeNumericString(ui.LegalMonetaryTotal.PrepaidAmount.Value))
		if err != nil {
			return err
		}
		payment.Advances = append(payment.Advances, &pay.Record{
			Amount:      totalPrepaid,
			Description: "Prepaid Amount",
		})
	}
	return nil
}

// OIOUBL: fixes up CreditTransfer via goblCreditTransfer and reverses the
// Giro/FIK/IBAN payment-channel handling via goblPaymentChannel.
func goblInvoiceInstructions(out *bill.Invoice, paymentMeans *PaymentMeans) *pay.Instructions {
	instructions := ubl.GoblInvoiceInstructions(out, paymentMeans)
	if paymentMeans.PayeeFinancialAccount != nil {
		instructions.CreditTransfer = goblCreditTransfer(paymentMeans)
	}
	goblPaymentChannel(instructions, paymentMeans)
	return instructions
}

// goblPaymentChannel reverses the OIOUBL payment-channel handling: pin to
// MeansKeyOther so EN 16931 keeps the explicit cbc:PaymentMeansCode, and for
// Giro/FIK recover the payment number and FIK creditor account from the wire.
func goblPaymentChannel(instr *pay.Instructions, paymentMeans *PaymentMeans) {
	if paymentMeans.PaymentChannelCode == nil {
		return
	}
	switch paymentMeans.PaymentChannelCode.Value {
	case paymentChannelIBAN, paymentChannelDKBank, paymentChannelNemKonto:
		instr.Key = pay.MeansKeyOther
		return
	case paymentChannelGiro, paymentChannelFIK:
	default:
		// An unsupported channel (BBAN, SE:BANKGIRO, ZZZ, …) is dropped rather
		// than round-tripped; applyPayment can't rederive it either.
		return
	}

	instr.Key = pay.MeansKeyOther
	// The generic path put the kortart in Ref; the real payment number is the InstructionID.
	instr.Ref = ""
	if paymentMeans.InstructionID != nil {
		instr.Ref = cbc.Code(ubl.CleanString(*paymentMeans.InstructionID))
	}
	if paymentMeans.CreditAccount != nil && paymentMeans.CreditAccount.AccountID != "" {
		instr.CreditTransfer = []*pay.CreditTransfer{{Number: paymentMeans.CreditAccount.AccountID}}
	}
}

// goblCreditTransfer fixes the base's branch: for DK:BANK it's really the
// bank reg. nr., not a BIC; for IBAN the BIC nests under FinancialInstitution.
func goblCreditTransfer(paymentMeans *PaymentMeans) []*pay.CreditTransfer {
	ct := ubl.GoblCreditTransfer(paymentMeans)
	if len(ct) == 0 {
		return ct
	}
	if isDKBankChannel(paymentMeans) {
		if ct[0].BIC != "" {
			ct[0].Branch = &org.Address{Label: ct[0].BIC}
			ct[0].BIC = ""
		}
		return ct
	}
	if ct[0].BIC != "" {
		return ct
	}
	if branch := paymentMeans.PayeeFinancialAccount.FinancialInstitutionBranch; branch != nil &&
		branch.FinancialInstitution != nil && branch.FinancialInstitution.ID != nil {
		ct[0].BIC = ubl.CleanString(*branch.FinancialInstitution.ID)
	}
	return ct
}

func isDKBankChannel(paymentMeans *PaymentMeans) bool {
	return paymentMeans.PaymentChannelCode != nil &&
		paymentMeans.PaymentChannelCode.Value == paymentChannelDKBank
}
