package oioubl

import (
	ubl "github.com/invopop/gobl.ubl"

	"github.com/invopop/gobl/bill"
)

// applySchemes stamps OIOUBL's scheme/list identifiers, header fields
// the base drops, and payment channel codes.
func (ui *Invoice) applySchemes(inv *bill.Invoice) {
	ui.UBLVersionID = Version
	ui.CustomizationID = CustomizationID
	ui.ProfileID = newProfileID()
	if !inv.UUID.IsZero() {
		ui.UUID = inv.UUID.String()
	}
	if inv.Ordering != nil && inv.Ordering.Cost != "" {
		ui.AccountingCost = inv.Ordering.Cost.String()
	}
	ui.applyOrderingRefs(inv)
	applyTypeCode(ui.InvoiceTypeCode)
	applyTypeCode(ui.CreditNoteTypeCode)
	ui.applyBillingReference()
	ui.applyAttachments()
	ui.applyPayment(inv)
}

// newProfileID builds the cbc:ProfileID with the profileid-1.2 scheme
// attributes; the EN 16931 base carries no ProfileID at all.
func newProfileID() *ubl.IDType {
	schemeID := schemeProfileV12
	schemeAgencyID := agencyID
	return &ubl.IDType{
		SchemeID:       &schemeID,
		SchemeAgencyID: &schemeAgencyID,
		Value:          ProfileID,
	}
}

// applyOrderingRefs puts back details the base drops: the order and contract
// issue dates, and the preceding documents' UUIDs.
func (ui *Invoice) applyOrderingRefs(inv *bill.Invoice) {
	if o := inv.Ordering; o != nil && len(o.Purchases) > 0 && ui.OrderReference != nil {
		if d := o.Purchases[0].IssueDate; d != nil {
			ui.OrderReference.IssueDate = formatDate(*d)
		}
	}
	if o := inv.Ordering; o != nil {
		for i, c := range o.Contracts {
			if i >= len(ui.ContractDocumentReference) {
				break
			}
			if c == nil {
				continue
			}
			if c.IssueDate != nil {
				ui.ContractDocumentReference[i].IssueDate = formatDate(*c.IssueDate)
			}
			if !c.UUID.IsZero() {
				ui.ContractDocumentReference[i].UUID = c.UUID.String()
			}
		}
	}
	for i, ref := range inv.Preceding {
		if i >= len(ui.BillingReference) {
			break
		}
		br := ui.BillingReference[i]
		if ref.UUID.IsZero() || br == nil || br.InvoiceDocumentReference == nil {
			continue
		}
		br.InvoiceDocumentReference.UUID = ref.UUID.String()
	}
}

func applyTypeCode(t *ubl.IDType) {
	if t == nil {
		return
	}
	t.ListID = ptr(codelistInvoiceType)
	t.ListAgencyID = ptr(agencyID)
}

// applyBillingReference drops the DocumentTypeCode from billing references;
// OIOUBL excludes it on both invoices and credit notes (F-LIB172).
func (ui *Invoice) applyBillingReference() {
	for i := range ui.BillingReference {
		if ref := ui.BillingReference[i]; ref != nil && ref.InvoiceDocumentReference != nil {
			ref.InvoiceDocumentReference.DocumentTypeCode = ""
		}
	}
}

// applyAttachments stamps a DocumentType on every additional document
// reference; OIOUBL requires DocumentType or DocumentTypeCode on each (F-LIB092).
func (ui *Invoice) applyAttachments() {
	for i := range ui.AdditionalDocumentReference {
		ref := &ui.AdditionalDocumentReference[i]
		if ref.DocumentType == "" && ref.DocumentTypeCode == "" {
			ref.DocumentType = "Supporting Document"
		}
	}
}
