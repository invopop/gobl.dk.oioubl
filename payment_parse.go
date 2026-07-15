package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/tax"
)

func (ui *Invoice) goblAddPayment(out *bill.Invoice) error {
	payment := &bill.PaymentDetails{}

	if ui.PayeeParty != nil {
		payment.Payee = goblParty(ui.PayeeParty)
	}

	if ui.PaymentTerms != nil {
		payment.Terms = &pay.Terms{
			Notes: ubl.CleanString(ui.PaymentTerms.Note),
		}
	}

	var dueDate string
	if ui.CreditNoteTypeCode == nil {
		dueDate = ui.DueDate
	}
	// OIOUBL (and credit notes, with no root DueDate) carry the due date on the
	// payment means; read it back when the root is absent.
	if dueDate == "" && len(ui.PaymentMeans) > 0 && ui.PaymentMeans[0].PaymentDueDate != nil {
		dueDate = *ui.PaymentMeans[0].PaymentDueDate
	}

	if dueDate != "" {
		d, err := ubl.ParseDate(dueDate)
		if err != nil {
			return err
		}
		if payment.Terms == nil {
			payment.Terms = &pay.Terms{}
		}
		payment.Terms.DueDates = append(payment.Terms.DueDates, &pay.DueDate{
			Date: &d,
		})
	}

	// A single due date takes 100%.
	if payment.Terms != nil && len(payment.Terms.DueDates) == 1 {
		percent, err := num.PercentageFromString("100%")
		if err != nil {
			return err
		}
		payment.Terms.DueDates[0].Percent = &percent
	}

	if len(ui.PaymentMeans) > 0 {
		payment.Instructions = goblInvoiceInstructions(out, &ui.PaymentMeans[0])
	}

	// OIOUBL records each advance as a cac:PrepaidPayment (F-INV131); reconstruct
	// them individually, or recover a single advance from a total-only PrepaidAmount.
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

	if payment.Payee != nil || payment.Terms != nil || payment.Instructions != nil || len(payment.Advances) > 0 {
		out.Payment = payment
	}
	return nil
}

// OIOUBL: also runs goblPaymentChannel to reverse the Giro/FIK/IBAN payment-channel handling.
func goblInvoiceInstructions(out *bill.Invoice, paymentMeans *PaymentMeans) *pay.Instructions {
	instructions := &pay.Instructions{
		Key: ubl.GoblPaymentMeansCode(paymentMeans.PaymentMeansCode.Value),
		Ext: tax.ExtensionsOf(cbc.CodeMap{
			untdid.ExtKeyPaymentMeans: cbc.Code(paymentMeans.PaymentMeansCode.Value),
		}),
	}

	if paymentMeans.PaymentMeansCode.Name != nil {
		instructions.Detail = ubl.CleanString(*paymentMeans.PaymentMeansCode.Name)
	}

	if paymentMeans.PaymentID != nil {
		instructions.Ref = cbc.Code(*paymentMeans.PaymentID)
	}

	if paymentMeans.PayeeFinancialAccount != nil {
		instructions.CreditTransfer = goblCreditTransfer(paymentMeans)
	}
	if paymentMeans.PaymentMandate != nil {
		instructions.DirectDebit = ubl.GoblInvoiceDirectDebit(out, paymentMeans)
	}
	if paymentMeans.CardAccount != nil {
		instructions.Card = ubl.GoblCard(paymentMeans)
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
	case paymentChannelIBAN, paymentChannelNemKonto:
		instr.Key = pay.MeansKeyOther
		return
	case paymentChannelGiro, paymentChannelFIK:
	default:
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

// goblCreditTransfer reads the BIC from FinancialInstitution/ID when the base
// doesn't find one on the branch itself (OIOUBL strips it there for IBAN
// accounts and nests it under FinancialInstitution instead, F-LIB295).
func goblCreditTransfer(paymentMeans *PaymentMeans) []*pay.CreditTransfer {
	ct := ubl.GoblCreditTransfer(paymentMeans)
	if len(ct) == 0 || ct[0].BIC != "" {
		return ct
	}
	if branch := paymentMeans.PayeeFinancialAccount.FinancialInstitutionBranch; branch != nil &&
		branch.FinancialInstitution != nil && branch.FinancialInstitution.ID != nil {
		ct[0].BIC = ubl.CleanString(*branch.FinancialInstitution.ID)
	}
	return ct
}
