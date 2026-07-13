package dkoioubl

import (
	"regexp"
	"strings"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/tax"
)

var (
	paymentMeansMap = map[string]cbc.Key{
		"10": pay.MeansKeyCash,
		"20": pay.MeansKeyCheque,
		"30": pay.MeansKeyCreditTransfer,
		"42": pay.MeansKeyDebitTransfer,
		"48": pay.MeansKeyCard,
		"49": pay.MeansKeyDirectDebit,
		"58": pay.MeansKeyCreditTransfer.With(pay.MeansKeySEPA),
		"59": pay.MeansKeyDirectDebit.With(pay.MeansKeySEPA),
	}

	// ibanRegex matches IBAN-like values: 2+ letters then alphanumerics, spaces allowed.
	ibanRegex = regexp.MustCompile(`^[A-Z]{2,}\s*[0-9A-Z\s]+$`)
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

func goblInvoiceInstructions(out *bill.Invoice, paymentMeans *PaymentMeans) *pay.Instructions {
	instructions := &pay.Instructions{
		Key: goblPaymentMeansCode(paymentMeans.PaymentMeansCode.Value),
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
		instructions.DirectDebit = goblInvoiceDirectDebit(out, paymentMeans)
	}
	if paymentMeans.CardAccount != nil {
		instructions.Card = goblCard(paymentMeans)
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
	case paymentChannelIBAN:
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

func goblCreditTransfer(paymentMeans *PaymentMeans) []*pay.CreditTransfer {
	creditTransfer := &pay.CreditTransfer{}
	account := paymentMeans.PayeeFinancialAccount

	if account.ID != nil {
		id := ubl.CleanString(*account.ID)
		if isIBAN(id) {
			creditTransfer.IBAN = id
		} else {
			creditTransfer.Number = id
		}
	}
	if account.Name != nil {
		creditTransfer.Name = ubl.CleanString(*account.Name)
	}
	if branch := account.FinancialInstitutionBranch; branch != nil {
		// OIOUBL strips the BIC off the branch ID for IBAN accounts (F-LIB295),
		// nesting it under FinancialInstitution/ID instead.
		if branch.ID != nil {
			creditTransfer.BIC = ubl.CleanString(*branch.ID)
		} else if branch.FinancialInstitution != nil && branch.FinancialInstitution.ID != nil {
			creditTransfer.BIC = ubl.CleanString(*branch.FinancialInstitution.ID)
		}
	}

	return []*pay.CreditTransfer{creditTransfer}
}

// isIBAN reports whether s looks like an IBAN: 2+ letters then alphanumerics (spaces allowed).
func isIBAN(s string) bool {
	s = strings.ToUpper(strings.TrimSpace(s))
	return ibanRegex.MatchString(s)
}

func goblInvoiceDirectDebit(out *bill.Invoice, paymentMeans *PaymentMeans) *pay.DirectDebit {
	directDebit := &pay.DirectDebit{}

	directDebit.Ref = paymentMeans.PaymentMandate.ID.Value
	if paymentMeans.PaymentMandate.PayerFinancialAccount != nil && paymentMeans.PaymentMandate.PayerFinancialAccount.ID != nil {
		directDebit.Account = *paymentMeans.PaymentMandate.PayerFinancialAccount.ID
	}
	seller := out.Supplier
	if seller != nil {
		for _, id := range seller.Identities {
			if id.Label == "SEPA" {
				directDebit.Creditor = id.Code.String()
				break
			}
		}
	}
	payment := out.Payment
	if payment != nil && payment.Payee != nil {
		payee := payment.Payee
		for _, id := range payee.Identities {
			if id.Label == "SEPA" {
				directDebit.Creditor = id.Code.String()
				break
			}
		}
	}
	return directDebit
}

func goblCard(paymentMeans *PaymentMeans) *pay.Card {
	card := &pay.Card{}
	if paymentMeans.CardAccount.PrimaryAccountNumberID != nil {
		pan := *paymentMeans.CardAccount.PrimaryAccountNumberID
		if len(pan) >= 4 {
			pan = pan[len(pan)-4:]
		}
		card.Last4 = pan
	}
	if paymentMeans.CardAccount.HolderName != nil {
		card.Holder = *paymentMeans.CardAccount.HolderName
	}
	return card
}

// goblPaymentMeansCode maps UBL payment means to GOBL equivalent.
func goblPaymentMeansCode(code string) cbc.Key {
	if val, ok := paymentMeansMap[code]; ok {
		return val
	}
	return pay.MeansKeyAny
}
