// Package dkoioubl converts GOBL documents to and from OIOUBL 2.1, the Danish
// NemHandel profile of UBL 2.1. The generic UBL plumbing (serialization,
// namespaces, version) is shared with github.com/invopop/gobl.ubl; everything
// OIOUBL-specific lives here, alongside the dk-oioubl GOBL addon in the addon
// subpackage.
package dkoioubl

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"

	"github.com/invopop/gobl"
	oioubl "github.com/invopop/gobl.dk.oioubl/addon"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/xmlctx"
)

var (
	// ErrUnknownDocumentType is returned when the document type
	// is not recognized during parsing.
	ErrUnknownDocumentType = fmt.Errorf("unknown document type")

	// ErrUnsupportedDocumentType is returned when the document type
	// is not supported for conversion.
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

// Addons lists the GOBL addons an OIOUBL document requires; they are ensured
// on conversion and stamped onto parsed documents.
var Addons = []cbc.Key{oioubl.V2}

// GetVESID returns the phive VESID for the given invoice.
func GetVESID(inv *bill.Invoice) string {
	if inv.Type.In(bill.InvoiceTypeCreditNote) {
		return VESIDCreditNote
	}
	return VESIDInvoice
}

// OIOUBL 2.1 code-list and scheme identifiers. These are the schemeID/listID
// attribute values the schematron expects (agency 320 throughout); centralised
// so every OIOUBL wire identifier has a single source of truth.
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

// Parse parses a raw OIOUBL document and returns the underlying Go struct. The
// returned value is an *Invoice (for both Invoice and CreditNote documents),
// whose Convert method returns the GOBL envelope.
func Parse(data []byte) (any, error) {
	ns, err := extractRootNamespace(data)
	if err != nil {
		return nil, err
	}

	switch ns {
	case NamespaceUBLInvoice, NamespaceUBLCreditNote:
		in := new(Invoice)
		if err := xmlctx.Unmarshal(data, in, xmlctx.WithNamespaces(map[string]string{
			"":     ns,
			"cbc":  NamespaceCBC,
			"cac":  NamespaceCAC,
			"qdt":  NamespaceQDT,
			"udt":  NamespaceUDT,
			"ccts": NamespaceCCTS,
			"xsi":  NamespaceXSI,
			"ext":  NamespaceEXT,
		})); err != nil {
			return nil, err
		}
		return in, nil

	default:
		return nil, ErrUnknownDocumentType
	}
}

// Convert takes a GOBL envelope containing a bill.Invoice and converts it to an
// OIOUBL *Invoice (or CreditNote on the wire).
func Convert(env *gobl.Envelope) (any, error) {
	switch doc := env.Extract().(type) {
	case *bill.Invoice:
		// Check and add missing addons
		if err := ensureAddons(env, Addons); err != nil {
			return nil, err
		}
		// Removes included taxes as they are not supported in UBL
		if err := doc.RemoveIncludedTaxes(); err != nil {
			return nil, fmt.Errorf("cannot convert invoice with included taxes: %w", err)
		}
		return newInvoice(doc)
	default:
		return nil, ErrUnsupportedDocumentType
	}
}

// ConvertInvoice is a convenience function that converts a GOBL envelope
// containing an invoice into an OIOUBL Invoice or CreditNote document.
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

// ensureAddons checks that the invoice carries all required addons and adds the
// missing ones, recalculating and revalidating the envelope so any rule the
// newly added addon enforces is surfaced (as a *gobl.Error carrying the faults).
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

func extractRootNamespace(data []byte) (string, error) {
	dc := xml.NewDecoder(bytes.NewReader(data))
	for {
		tk, err := dc.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("error parsing XML: %w", err)
		}
		switch t := tk.(type) {
		case xml.StartElement:
			return t.Name.Space, nil // Extract and return the namespace
		}
	}
	return "", ErrUnknownDocumentType
}

// Bytes returns the raw XML of the document including the XML header,
// serialized via gobl.ubl. A credit note carrying a cbc:TaxPointDate is
// reordered to the CreditNote XSD sequence — see reorderCreditNoteTaxPointDate.
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

// BytesCompact returns the raw XML of the document without indentation,
// including the XML header.
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

// creditNoteNeedsTaxPointDateReorder reports whether in is a credit note
// carrying a cbc:TaxPointDate, which the CreditNote XSD sequences differently
// from the shared Invoice struct — see reorderCreditNoteTaxPointDate.
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

// reorderCreditNoteTaxPointDate moves cbc:TaxPointDate ahead of
// cbc:CreditNoteTypeCode to match the CreditNote XSD sequence. Invoice and
// CreditNote share one Go struct, and encoding/xml can neither vary field order
// per struct nor survive a decode/re-encode (it mangles the cac:/cbc: prefixes),
// so the fix edits the marshaled bytes directly.
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

	// Re-insert it before the type code, reusing that line's leading whitespace
	// (empty for the compact, non-indented output).
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
