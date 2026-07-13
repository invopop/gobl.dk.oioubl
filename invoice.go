package dkoioubl

import (
	"encoding/xml"

	ubl "github.com/invopop/gobl.ubl"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	cur "github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/tax"
)

// rootNameCreditNote is the local name of a UBL CreditNote root element.
const rootNameCreditNote = "CreditNote"

// Adapted from gobl.ubl (ublInvoice); OIOUBL: stamps the OIOUBL customization/profile/scheme IDs and the credit-note element/schema swap.
func newInvoice(inv *bill.Invoice) (*Invoice, error) {
	tc, err := getTypeCode(inv)
	if err != nil {
		return nil, err
	}

	out := &Invoice{
		XMLName:                 xml.Name{Local: "Invoice"},
		CACNamespace:            ubl.NamespaceCAC,
		CBCNamespace:            ubl.NamespaceCBC,
		QDTNamespace:            ubl.NamespaceQDT,
		UDTNamespace:            ubl.NamespaceUDT,
		UBLNamespace:            ubl.NamespaceUBLInvoice,
		CCTSNamespace:           ubl.NamespaceCCTS,
		XSINamespace:            ubl.NamespaceXSI,
		EXTNamespace:            ubl.NamespaceEXT,
		SchemaLocation:          ubl.SchemaLocationInvoice,
		UBLVersionID:            Version,
		CustomizationID:         CustomizationID,
		ProfileID:               newProfileID(ProfileID),
		ID:                      ubl.InvoiceNumber(inv.Series, inv.Code),
		IssueDate:               ubl.FormatDate(inv.IssueDate),
		InvoiceTypeCode:         &IDType{Value: tc},
		DocumentCurrencyCode:    string(inv.Currency),
		AccountingSupplierParty: SupplierParty{Party: newParty(inv.Supplier)},
		AccountingCustomerParty: CustomerParty{Party: newParty(inv.Customer)},
	}

	// BR-53 needs an exchange rate when cbc:TaxCurrencyCode is present, and the
	// restated tax rides StandardRated subtotals only (F-LIB373 / F-INV018).
	if taxCurrency := inv.RegimeDef().Currency; taxCurrency != inv.Currency &&
		cur.MatchExchangeRate(inv.ExchangeRates, inv.Currency, taxCurrency) != nil &&
		hasStandardRated(inv) {
		out.TaxCurrencyCode = string(taxCurrency)
	}

	if inv.Ordering != nil && inv.Ordering.Cost != "" {
		out.AccountingCost = inv.Ordering.Cost.String()
	}

	if !inv.UUID.IsZero() {
		out.UUID = inv.UUID.String()
	}
	if out.ProfileID != nil {
		out.ProfileID.SchemeID = ptr(schemeProfileV12)
		out.ProfileID.SchemeAgencyID = ptr(agencyID)
	}

	if inv.Type.In(bill.InvoiceTypeCreditNote) {
		out.XMLName = xml.Name{Local: rootNameCreditNote}
		out.UBLNamespace = ubl.NamespaceUBLCreditNote
		out.SchemaLocation = ubl.SchemaLocationCrediteNote
		out.InvoiceTypeCode = nil
		out.CreditNoteTypeCode = &IDType{Value: tc}
	}

	// BT-7: VAT point date
	if inv.ValueDate != nil {
		out.TaxPointDate = ubl.FormatDate(*inv.ValueDate)
	}

	for _, note := range inv.Notes {
		if text := ubl.FormatNote(note); text != "" {
			out.Note = append(out.Note, text)
		}
	}

	out.addPreceding(inv.Preceding)
	out.addOrdering(inv.Ordering)
	out.addTaxPoint(inv.Tax)
	out.addCharges(inv)
	out.addTotals(inv)
	out.addLines(inv)
	(*ubl.Invoice)(out).AddAttachments(inv.Attachments)

	if err = out.addPayment(inv); err != nil {
		return nil, err
	}
	if d := newDelivery(inv.Delivery); d != nil {
		out.Delivery = []*Delivery{d}
	}

	applyTypeCode(out.InvoiceTypeCode)
	applyTypeCode(out.CreditNoteTypeCode)
	applyParty(out.AccountingSupplierParty.Party)
	applyParty(out.AccountingCustomerParty.Party)
	applyParty(out.PayeeParty)
	applyTaxRepParty(out.TaxRepresentativeParty)
	out.applyBillingReference()
	out.applyAttachments()
	out.applyTotals()

	return out, nil
}

// taxPointCodeMap maps GOBL tax point keys to UNTDID 2005 codes for UBL.
var taxPointCodeMap = map[cbc.Key]string{
	tax.PointIssue:    "3",
	tax.PointDelivery: "35",
	tax.PointPayment:  "432",
}

// Identical to gobl.ubl.
func (ui *Invoice) addTaxPoint(t *bill.Tax) {
	if t == nil || t.Point == cbc.KeyEmpty {
		return
	}
	code, ok := taxPointCodeMap[t.Point]
	if !ok {
		return
	}
	if len(ui.InvoicePeriod) == 0 {
		ui.InvoicePeriod = []Period{{}}
	}
	ui.InvoicePeriod[0].DescriptionCode = code
}

func newProfileID(profileID string) *IDType {
	if profileID == "" {
		return nil
	}
	return &IDType{Value: profileID}
}

// Identical to gobl.ubl.
func (ui *Invoice) getInvoiceTypeBasedOnXMLName() cbc.Key {
	switch ui.XMLName.Local {
	case rootNameCreditNote:
		return bill.InvoiceTypeCreditNote
	default:
		return bill.InvoiceTypeStandard
	}
}

func applyTypeCode(t *IDType) {
	if t == nil {
		return
	}
	t.ListID = ptr(listInvoiceType)
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
