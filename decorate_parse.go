package oioubl

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/uuid"
)

// addOIOUBLDetails puts back what the generic parse has no field for, reading
// it off the original document.
func (ui *Invoice) addOIOUBLDetails(inv *bill.Invoice, docExcises []exciseDuty, lineExcises map[int][]exciseDuty, vatPercents map[string]string) {
	addExciseCharges(inv, docExcises, lineExcises, vatPercents)

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

	addOrderingDetails(inv, ui)
}

func addOrderingDetails(inv *bill.Invoice, ui *Invoice) {
	if ui.AccountingCost != "" {
		if inv.Ordering == nil {
			inv.Ordering = new(bill.Ordering)
		}
		inv.Ordering.Cost = cbc.Code(ui.AccountingCost)
	}
	if ui.OrderReference != nil && ui.OrderReference.IssueDate != "" &&
		inv.Ordering != nil && len(inv.Ordering.Purchases) > 0 {
		if d, err := parseWireDate(ui.OrderReference.IssueDate); err == nil {
			inv.Ordering.Purchases[0].IssueDate = &d
		}
	}
	for i, ref := range ui.BillingReference {
		if i >= len(inv.Preceding) || ref == nil {
			continue
		}
		var uuidStr string
		switch {
		case ref.InvoiceDocumentReference != nil:
			uuidStr = ref.InvoiceDocumentReference.UUID
		case ref.CreditNoteDocumentReference != nil:
			uuidStr = ref.CreditNoteDocumentReference.UUID
		}
		if uuidStr == "" {
			continue
		}
		if u, err := uuid.Parse(uuidStr); err == nil {
			inv.Preceding[i].UUID = u
		}
	}
}
