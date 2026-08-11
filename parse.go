package oioubl

import (
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
// order's issue date and the preceding documents' UUIDs.
func (ui *Invoice) addOrderingDetails(inv *bill.Invoice) {
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
