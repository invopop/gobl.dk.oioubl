package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"

	"github.com/invopop/gobl/bill"
)

// applySchemeFlavor stamps the OIOUBL scheme/list/agency identifiers the
// schematron expects (UBLVersionID, CustomizationID/ProfileID, type code
// lists) along with the document-header fields the base drops, and adjusts
// the payment means' channel codes.
func (ui *Invoice) applySchemeFlavor(inv *bill.Invoice) {
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

// applyOrderingRefs restores reference fields the base builder drops: the
// order reference's issue date and the preceding documents' UUIDs.
func (ui *Invoice) applyOrderingRefs(inv *bill.Invoice) {
	if o := inv.Ordering; o != nil && len(o.Purchases) > 0 && ui.OrderReference != nil {
		if d := o.Purchases[0].IssueDate; d != nil {
			ui.OrderReference.IssueDate = ubl.FormatDate(*d)
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
	listID := listInvoiceType
	listAgencyID := agencyID
	t.ListID = &listID
	t.ListAgencyID = &listAgencyID
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
