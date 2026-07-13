// Package dkoioubl converts GOBL documents to and from OIOUBL 2.1, the Danish
// NemHandel profile of UBL 2.1. Generic UBL plumbing is shared with gobl.ubl;
// OIOUBL-specifics live here alongside the dk-oioubl addon subpackage.
package dkoioubl

import (
	"bytes"
	"fmt"

	"github.com/invopop/gobl"
	oioubl "github.com/invopop/gobl.dk.oioubl/addon"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
)

var (
	// ErrUnknownDocumentType is returned when the document type is not recognized during parsing.
	ErrUnknownDocumentType = fmt.Errorf("unknown document type")

	// ErrUnsupportedDocumentType is returned when the document type is not supported for conversion.
	ErrUnsupportedDocumentType = fmt.Errorf("unsupported document type")
)

// Version is the UBL version of the generated documents.
const Version = ubl.Version

// OIOUBL 2.1 document identification.
const (
	// CustomizationID identifies OIOUBL 2.1 documents.
	CustomizationID = "OIOUBL-2.1"
	// ProfileID is the NESUBL billing profile invoices and credit notes ride.
	ProfileID = "urn:www.nesubl.eu:profiles:profile5:ver2.0"
)

// VESIDs for phive validation of each document type.
const (
	VESIDInvoice    = "dk.oioubl:invoice:1.17.2"
	VESIDCreditNote = "dk.oioubl:credit-note:1.17.2"
)

// Addons lists the GOBL addons an OIOUBL document requires.
var Addons = []cbc.Key{oioubl.V2}

// GetVESID returns the phive VESID for the given invoice.
func GetVESID(inv *bill.Invoice) string {
	if inv.Type.In(bill.InvoiceTypeCreditNote) {
		return VESIDCreditNote
	}
	return VESIDInvoice
}

// OIOUBL code-list and scheme identifiers the schematron expects (agency 320 throughout).
const (
	agencyID = "320"

	schemeTaxCategory = "urn:oioubl:id:taxcategoryid-1.1"
	schemeTaxScheme   = "urn:oioubl:id:taxschemeid-1.1"
	schemeProfileV12  = "urn:oioubl:id:profileid-1.2"

	listInvoiceType    = "urn:oioubl:codelist:invoicetypecode-1.1"
	listPaymentChannel = "urn:oioubl:codelist:paymentchannelcode-1.1"
	listAddressFormat  = "urn:oioubl:codelist:addressformatcode-1.1"
	listTaxType        = "urn:oioubl:codelist:taxtypecode-1.1"
)

// Parse parses a raw OIOUBL document into an *Invoice whose Convert method returns the GOBL envelope.
func Parse(data []byte) (any, error) {
	doc, err := ubl.Parse(data)
	if err != nil {
		return nil, err
	}
	if in, ok := doc.(*ubl.Invoice); ok {
		return (*Invoice)(in), nil
	}
	return nil, ErrUnknownDocumentType
}

// Convert converts a GOBL envelope's bill.Invoice into an OIOUBL *Invoice.
func Convert(env *gobl.Envelope) (any, error) {
	switch doc := env.Extract().(type) {
	case *bill.Invoice:
		if err := ensureAddons(env, Addons); err != nil {
			return nil, err
		}
		// UBL does not support included taxes.
		if err := doc.RemoveIncludedTaxes(); err != nil {
			return nil, fmt.Errorf("cannot convert invoice with included taxes: %w", err)
		}
		return newInvoice(doc)
	default:
		return nil, ErrUnsupportedDocumentType
	}
}

// ConvertInvoice converts a GOBL envelope to an OIOUBL *Invoice.
func ConvertInvoice(env *gobl.Envelope) (*Invoice, error) {
	doc, err := Convert(env)
	if err != nil {
		return nil, err
	}
	inv, ok := doc.(*Invoice)
	if !ok {
		return nil, fmt.Errorf("expected invoice, got %T", doc)
	}
	return inv, nil
}

// ensureAddons adds any missing required addons, then recalculates and
// revalidates so their rules surface as faults.
func ensureAddons(env *gobl.Envelope, required []cbc.Key) error {
	if len(required) == 0 {
		return nil
	}

	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return ErrUnsupportedDocumentType
	}

	var missing []cbc.Key
	existing := inv.GetAddons()
	for _, addon := range required {
		if !addon.In(existing...) {
			missing = append(missing, addon)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	inv.SetAddons(append(existing, missing...)...)
	if err := env.Calculate(); err != nil {
		return err
	}
	return env.Validate()
}

// Bytes returns the document's XML with header (reordering credit-note TaxPointDate, see reorderCreditNoteTaxPointDate).
func Bytes(in any) ([]byte, error) {
	b, err := ubl.Bytes(in)
	if err != nil {
		return nil, err
	}
	if creditNoteNeedsTaxPointDateReorder(in) {
		b = reorderCreditNoteTaxPointDate(b)
	}
	return b, nil
}

// BytesCompact returns the document's XML with header, without indentation.
func BytesCompact(in any) ([]byte, error) {
	b, err := ubl.BytesCompact(in)
	if err != nil {
		return nil, err
	}
	if creditNoteNeedsTaxPointDateReorder(in) {
		b = reorderCreditNoteTaxPointDate(b)
	}
	return b, nil
}

func creditNoteNeedsTaxPointDateReorder(in any) bool {
	var inv *Invoice
	switch v := in.(type) {
	case *Invoice:
		inv = v
	case Invoice:
		inv = &v
	default:
		return false
	}
	return inv != nil && inv.XMLName.Local == rootNameCreditNote && inv.TaxPointDate != ""
}

// reorderCreditNoteTaxPointDate moves cbc:TaxPointDate ahead of cbc:CreditNoteTypeCode for the CreditNote XSD sequence (one shared struct, so it edits the marshaled bytes).
func reorderCreditNoteTaxPointDate(b []byte) []byte {
	const (
		open     = "<cbc:TaxPointDate>"
		closeTag = "</cbc:TaxPointDate>"
		typeCode = "<cbc:CreditNoteTypeCode"
	)

	tpd := bytes.Index(b, []byte(open))
	tc := bytes.Index(b, []byte(typeCode))
	if tpd < 0 || tc < 0 || tpd < tc {
		return b // type code absent, or already correctly ordered
	}
	rel := bytes.Index(b[tpd:], []byte(closeTag))
	if rel < 0 {
		return b
	}
	elemEnd := tpd + rel + len(closeTag)
	elem := append([]byte(nil), b[tpd:elemEnd]...)

	// Drop the element together with the newline + indent that preceded it.
	cut := tpd
	for cut > 0 && (b[cut-1] == ' ' || b[cut-1] == '\t') {
		cut--
	}
	if cut > 0 && b[cut-1] == '\n' {
		cut--
	}
	rest := append(append([]byte(nil), b[:cut]...), b[elemEnd:]...)

	// Re-insert it before the type code, reusing that line's leading whitespace.
	tc = bytes.Index(rest, []byte(typeCode))
	indentStart := tc
	for indentStart > 0 && (rest[indentStart-1] == ' ' || rest[indentStart-1] == '\t') {
		indentStart--
	}
	if indentStart > 0 && rest[indentStart-1] == '\n' {
		indentStart--
	}
	sep := rest[indentStart:tc]

	out := make([]byte, 0, len(rest)+len(elem)+len(sep))
	out = append(out, rest[:tc]...)
	out = append(out, elem...)
	out = append(out, sep...)
	out = append(out, rest[tc:]...)
	return out
}
