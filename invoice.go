package dkoioubl

import (
	"encoding/xml"
	"fmt"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	cur "github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/tax"
)

// UBL invoice and credit-note root namespaces.
const (
	NamespaceUBLInvoice    = "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
	NamespaceUBLCreditNote = "urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2"

	// rootNameCreditNote is the local name of a UBL CreditNote root element.
	rootNameCreditNote = "CreditNote"
)

// Schema location constants
const (
	SchemaLocationInvoice    = "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2 http://docs.oasis-open.org/ubl/os-UBL-2.1/xsd/maindoc/UBL-Invoice-2.1.xsd"
	SchemaLocationCreditNote = "urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2 https://docs.oasis-open.org/ubl/os-UBL-2.1/xsd/maindoc/UBL-CreditNote-2.1.xsd"
)

// Invoice represents the root element of an OIOUBL Invoice **or** Credit Note;
// the structures between the two types are so similar, that it doesn't make
// much sense to separate.
type Invoice struct {
	// Attributes
	XMLName        xml.Name
	CACNamespace   string `xml:"xmlns:cac,attr"`
	CBCNamespace   string `xml:"xmlns:cbc,attr"`
	QDTNamespace   string `xml:"xmlns:qdt,attr"`
	UDTNamespace   string `xml:"xmlns:udt,attr"`
	CCTSNamespace  string `xml:"xmlns:ccts,attr"`
	UBLNamespace   string `xml:"xmlns,attr"`
	XSINamespace   string `xml:"xmlns:xsi,attr"`
	EXTNamespace   string `xml:"xmlns:ext,attr"`
	SchemaLocation string `xml:"xsi:schemaLocation,attr,omitempty"`

	UBLVersionID       string  `xml:"cbc:UBLVersionID,omitempty"`
	CustomizationID    string  `xml:"cbc:CustomizationID,omitempty"`
	ProfileID          *IDType `xml:"cbc:ProfileID,omitempty"`
	ProfileExecutionID string  `xml:"cbc:ProfileExecutionID,omitempty"`
	ID                 string  `xml:"cbc:ID"`
	CopyIndicator      bool    `xml:"cbc:CopyIndicator,omitempty"`
	UUID               string  `xml:"cbc:UUID,omitempty"`
	IssueDate          string  `xml:"cbc:IssueDate"`
	IssueTime          string  `xml:"cbc:IssueTime,omitempty"`
	DueDate            string  `xml:"cbc:DueDate,omitempty"`

	InvoiceTypeCode    *IDType `xml:"cbc:InvoiceTypeCode,omitempty"`
	CreditNoteTypeCode *IDType `xml:"cbc:CreditNoteTypeCode,omitempty"`

	Note                           []string            `xml:"cbc:Note,omitempty"`
	TaxPointDate                   string              `xml:"cbc:TaxPointDate,omitempty"`
	DocumentCurrencyCode           string              `xml:"cbc:DocumentCurrencyCode,omitempty"`
	TaxCurrencyCode                string              `xml:"cbc:TaxCurrencyCode,omitempty"`
	PricingCurrencyCode            string              `xml:"cbc:PricingCurrencyCode,omitempty"`
	PaymentCurrencyCode            string              `xml:"cbc:PaymentCurrencyCode,omitempty"`
	PaymentAlternativeCurrencyCode string              `xml:"cbc:PaymentAlternativeCurrencyCode,omitempty"`
	AccountingCost                 string              `xml:"cbc:AccountingCost,omitempty"`
	LineCountNumeric               int                 `xml:"cbc:LineCountNumeric,omitempty"`
	BuyerReference                 string              `xml:"cbc:BuyerReference,omitempty"`
	InvoicePeriod                  []Period            `xml:"cac:InvoicePeriod,omitempty"`
	OrderReference                 *OrderReference     `xml:"cac:OrderReference,omitempty"`
	BillingReference               []*BillingReference `xml:"cac:BillingReference,omitempty"`
	DespatchDocumentReference      []Reference         `xml:"cac:DespatchDocumentReference,omitempty"`
	ReceiptDocumentReference       []Reference         `xml:"cac:ReceiptDocumentReference,omitempty"`
	StatementDocumentReference     []Reference         `xml:"cac:StatementDocumentReference,omitempty"`
	OriginatorDocumentReference    []Reference         `xml:"cac:OriginatorDocumentReference,omitempty"`
	ContractDocumentReference      []Reference         `xml:"cac:ContractDocumentReference,omitempty"`
	AdditionalDocumentReference    []Reference         `xml:"cac:AdditionalDocumentReference,omitempty"`
	ProjectReference               []ProjectReference  `xml:"cac:ProjectReference,omitempty"`
	AccountingSupplierParty        SupplierParty       `xml:"cac:AccountingSupplierParty"`
	AccountingCustomerParty        CustomerParty       `xml:"cac:AccountingCustomerParty"`
	PayeeParty                     *Party              `xml:"cac:PayeeParty,omitempty"`
	BuyerCustomerParty             *CustomerParty      `xml:"cac:BuyerCustomerParty,omitempty"`
	SellerSupplierParty            *SupplierParty      `xml:"cac:SellerSupplierParty,omitempty"`
	TaxRepresentativeParty         *Party              `xml:"cac:TaxRepresentativeParty,omitempty"`
	Delivery                       []*Delivery         `xml:"cac:Delivery,omitempty"`
	DeliveryTerms                  *DeliveryTerms      `xml:"cac:DeliveryTerms,omitempty"`
	PaymentMeans                   []PaymentMeans      `xml:"cac:PaymentMeans,omitempty"`
	PaymentTerms                   *PaymentTerms       `xml:"cac:PaymentTerms,omitempty"`
	PrepaidPayment                 []PrepaidPayment    `xml:"cac:PrepaidPayment,omitempty"`
	AllowanceCharge                []AllowanceCharge   `xml:"cac:AllowanceCharge,omitempty"`
	TaxExchangeRate                *ExchangeRate       `xml:"cac:TaxExchangeRate,omitempty"`
	PricingExchangeRate            *ExchangeRate       `xml:"cac:PricingExchangeRate,omitempty"`
	PaymentExchangeRate            *ExchangeRate       `xml:"cac:PaymentExchangeRate,omitempty"`
	PaymentAlternativeExchangeRate *ExchangeRate       `xml:"cac:PaymentAlternativeExchangeRate,omitempty"`
	TaxTotal                       []TaxTotal          `xml:"cac:TaxTotal,omitempty"`
	WithholdingTaxTotal            []TaxTotal          `xml:"cac:WithholdingTaxTotal,omitempty"`
	LegalMonetaryTotal             MonetaryTotal       `xml:"cac:LegalMonetaryTotal"`
	InvoiceLines                   []InvoiceLine       `xml:"cac:InvoiceLine,omitempty"`
	CreditNoteLines                []InvoiceLine       `xml:"cac:CreditNoteLine,omitempty"`
}

func newInvoice(inv *bill.Invoice) (*Invoice, error) {
	tc, err := getTypeCode(inv)
	if err != nil {
		return nil, err
	}

	out := &Invoice{
		XMLName:                 xml.Name{Local: "Invoice"},
		CACNamespace:            NamespaceCAC,
		CBCNamespace:            NamespaceCBC,
		QDTNamespace:            NamespaceQDT,
		UDTNamespace:            NamespaceUDT,
		UBLNamespace:            NamespaceUBLInvoice,
		CCTSNamespace:           NamespaceCCTS,
		XSINamespace:            NamespaceXSI,
		EXTNamespace:            NamespaceEXT,
		SchemaLocation:          SchemaLocationInvoice,
		UBLVersionID:            Version,
		CustomizationID:         CustomizationID,
		ProfileID:               newProfileID(ProfileID),
		ID:                      invoiceNumber(inv.Series, inv.Code),
		IssueDate:               formatDate(inv.IssueDate),
		InvoiceTypeCode:         &IDType{Value: tc},
		DocumentCurrencyCode:    string(inv.Currency),
		AccountingSupplierParty: SupplierParty{Party: newParty(inv.Supplier)},
		AccountingCustomerParty: CustomerParty{Party: newParty(inv.Customer)},
	}

	// BR-53 requires an exchange rate whenever cbc:TaxCurrencyCode is present,
	// and OIOUBL carries the restated tax only on StandardRated subtotals
	// (F-LIB373), so it also needs one of those to satisfy F-INV018.
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
		// Invoices/credit notes carry profile5:ver2.0, valid from the
		// profileid-1.2 code list — which real NemHandel traffic uses.
		out.ProfileID.SchemeID = ptr(schemeProfileV12)
		out.ProfileID.SchemeAgencyID = ptr(agencyID)
	}

	if inv.Type.In(bill.InvoiceTypeCreditNote) {
		out.XMLName = xml.Name{Local: rootNameCreditNote}
		out.UBLNamespace = NamespaceUBLCreditNote
		out.SchemaLocation = SchemaLocationCreditNote
		out.InvoiceTypeCode = nil
		out.CreditNoteTypeCode = &IDType{Value: tc}
	}

	// BT-7: VAT point date
	if inv.ValueDate != nil {
		out.TaxPointDate = formatDate(*inv.ValueDate)
	}

	for _, note := range inv.Notes {
		if text := formatNote(note); text != "" {
			out.Note = append(out.Note, text)
		}
	}

	out.addPreceding(inv.Preceding)
	out.addOrdering(inv.Ordering)
	out.addTaxPoint(inv.Tax)
	out.addCharges(inv)
	out.addTotals(inv)
	out.addLines(inv)
	out.AddAttachments(inv.Attachments)

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

// taxPointKeyMap is the reverse mapping from UNTDID 2005 codes to GOBL tax point keys.
var taxPointKeyMap = map[string]cbc.Key{
	"3":   tax.PointIssue,
	"35":  tax.PointDelivery,
	"432": tax.PointPayment,
}

// addTaxPoint maps the GOBL tax point key (BT-8) to the UBL InvoicePeriod DescriptionCode.
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

func invoiceNumber(series cbc.Code, code cbc.Code) string {
	if series == "" {
		return code.String()
	}
	return fmt.Sprintf("%s-%s", series, code)
}

func newProfileID(profileID string) *IDType {
	if profileID == "" {
		return nil
	}
	return &IDType{Value: profileID}
}

// getInvoiceTypeBasedOnXMLName returns the invoice type based on the XML root
// name instead of gobl's invoice type key, since OIOUBL credit notes omit the
// type code.
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
