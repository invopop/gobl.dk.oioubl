package oioubl

import (
	"fmt"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/uuid"
)

// addGOBLDetails fills in the GOBL fields the generic parse leaves empty,
// reading them off the original OIOUBL document.
func (ui *Invoice) addGOBLDetails(inv *bill.Invoice, details oioublDetails) {
	addExciseCharges(inv, details)

	if len(ui.PaymentMeans) > 0 && inv.Payment != nil {
		addPaymentDetails(inv.Payment, &ui.PaymentMeans[0])
	}

	// The generic parser never reads cbc:UUID; without this GOBL invents a
	// fresh one on every Calculate.
	if u, err := uuid.Parse(ui.UUID); err == nil {
		inv.UUID = u
	}

	addPartyContact(inv.Supplier, ui.AccountingSupplierParty.Party)
	addPartyContact(inv.Customer, ui.AccountingCustomerParty.Party)
	if inv.Payment != nil {
		addPartyContact(inv.Payment.Payee, ui.PayeeParty)
	}
	recoverIdentityScheme(inv.Supplier, ui.AccountingSupplierParty.Party)
	recoverIdentityScheme(inv.Customer, ui.AccountingCustomerParty.Party)
	markLegalIdentity(inv.Supplier)
	markLegalIdentity(inv.Customer)

	// OIOUBL omits the credit-note type code; the root element decides.
	if ui.XMLName.Local == rootNameCreditNote && inv.Type == bill.InvoiceTypeOther {
		inv.Type = bill.InvoiceTypeCreditNote
	}

	// A CPR-only supplier has no tax ID, so the regime can't derive normally.
	if inv.Supplier == nil || inv.Supplier.TaxID == nil || inv.Supplier.TaxID.Country == "" {
		inv.SetRegime("DK")
	}

	ui.addOrderingDetails(inv)
	ui.addPayableRounding(inv)
}

// addPayableRounding carries OIOUBL's afrundingsbeløb, which the generic parse
// never reads. Calculate leaves Rounding alone precisely so it can be supplied.
func (ui *Invoice) addPayableRounding(inv *bill.Invoice) {
	wire := ui.LegalMonetaryTotal.PayableRoundingAmount
	if wire == nil {
		return
	}
	amount, err := num.AmountFromString(normalizeNumericString(wire.Value))
	if err != nil || amount.IsZero() {
		return
	}
	if inv.Totals == nil {
		inv.Totals = new(bill.Totals)
	}
	inv.Totals.Rounding = &amount
}

// addOrderingDetails restores the ordering fields the generic parse skips: the
// order's issue date and the preceding and contract documents' UUIDs.
func (ui *Invoice) addOrderingDetails(inv *bill.Invoice) {
	if ui.OrderReference != nil && ui.OrderReference.IssueDate != "" &&
		inv.Ordering != nil && len(inv.Ordering.Purchases) > 0 {
		if d, err := parseWireDate(ui.OrderReference.IssueDate); err == nil {
			inv.Ordering.Purchases[0].IssueDate = &d
		}
	}
	// Pair by document code, not by index: the generic parser skips a billing
	// reference it cannot resolve, and after one skip every index is off by one
	// and each UUID lands on the wrong document.
	for _, ref := range ui.BillingReference {
		if ref == nil {
			continue
		}
		wire := billingDocumentReference(ref)
		if wire == nil || wire.UUID == "" {
			continue
		}
		u, err := uuid.Parse(wire.UUID)
		if err != nil {
			continue
		}
		for _, p := range inv.Preceding {
			if p != nil && p.UUID.IsZero() && p.Code.String() == wire.ID.Value {
				p.UUID = u
				break
			}
		}
	}
	// Contract references are never skipped by the generic parser, so here the
	// index does identify the document.
	if inv.Ordering != nil {
		for i := range ui.ContractDocumentReference {
			if i >= len(inv.Ordering.Contracts) {
				break
			}
			if u, err := uuid.Parse(ui.ContractDocumentReference[i].UUID); err == nil {
				inv.Ordering.Contracts[i].UUID = u
			}
		}
	}
}

// billingDocumentReference returns whichever document reference a billing
// reference carries, mirroring the generic parser's resolution order.
func billingDocumentReference(ref *ubl.BillingReference) *ubl.Reference {
	switch {
	case ref.InvoiceDocumentReference != nil:
		return ref.InvoiceDocumentReference
	case ref.SelfBilledInvoiceDocumentReference != nil:
		return ref.SelfBilledInvoiceDocumentReference
	case ref.CreditNoteDocumentReference != nil:
		return ref.CreditNoteDocumentReference
	case ref.AdditionalDocumentReference != nil:
		return ref.AdditionalDocumentReference
	}
	return nil
}

// checkStatedPayable refuses a conversion whose bottom line differs from what
// the document itself states: the converter must not change what the customer
// owes. A prepaid document states its payable net of the prepayment, which
// GOBL keeps as Due.
func (ui *Invoice) checkStatedPayable(inv *bill.Invoice) error {
	wire := ui.LegalMonetaryTotal.PayableAmount
	if wire == nil || wire.Value == "" || inv.Totals == nil {
		return nil
	}
	stated, err := num.AmountFromString(normalizeNumericString(wire.Value))
	if err != nil {
		return fmt.Errorf("stated payable amount %q: %w", wire.Value, err)
	}
	computed := inv.Totals.Payable
	if inv.Totals.Due != nil {
		computed = *inv.Totals.Due
	}
	if computed.Compare(stated) != 0 {
		return fmt.Errorf("converted payable %s does not match the document's stated payable %s", computed.String(), stated.String())
	}
	return nil
}
