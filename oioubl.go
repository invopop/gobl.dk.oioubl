// Package dkoioubl converts GOBL documents to and from OIOUBL 2.1, the Danish
// NemHandel profile of UBL 2.1. Generic UBL plumbing is shared with gobl.ubl;
// OIOUBL-specifics live here alongside the dk-oioubl addon subpackage.
package dkoioubl

import (
	"fmt"

	"github.com/invopop/gobl"
	oioubl "github.com/invopop/gobl.dk.oioubl/addon"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
)

var (
	ErrUnknownDocumentType     = fmt.Errorf("unknown document type")
	ErrUnsupportedDocumentType = fmt.Errorf("unsupported document type")
)

const Version = ubl.Version

const (
	CustomizationID = "OIOUBL-2.1"
	ProfileID       = "urn:www.nesubl.eu:profiles:profile5:ver2.0"
)

const (
	VESIDInvoice    = "dk.oioubl:invoice:1.17.2"
	VESIDCreditNote = "dk.oioubl:credit-note:1.17.2"
)

var Addons = []cbc.Key{oioubl.V2}

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
	schemeTaxScheme   = "urn:oioubl:id:taxschemeid-1.5"
	schemeProfileV12  = "urn:oioubl:id:profileid-1.2"

	listInvoiceType    = "urn:oioubl:codelist:invoicetypecode-1.1"
	listPaymentChannel = "urn:oioubl:codelist:paymentchannelcode-1.1"
	listAddressFormat  = "urn:oioubl:codelist:addressformatcode-1.1"
	listTaxType        = "urn:oioubl:codelist:taxtypecode-1.1"
)

// Parse parses an OIOUBL document into an *Invoice; call its Convert for the GOBL envelope.
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

func Convert(env *gobl.Envelope) (any, error) {
	out, err := convertViaOverlay(env)
	if err != nil {
		return nil, err
	}
	return out, nil
}

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

// convertViaOverlay builds the OIOUBL document by generating gobl.ubl's base
// EN 16931 document and applying the OIOUBL flavor on top.
func convertViaOverlay(env *gobl.Envelope) (*Invoice, error) {
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return nil, ErrUnsupportedDocumentType
	}
	// Both the base builder and the flavor dereference the regime definition;
	// reject a regime-less invoice cleanly instead of panicking downstream.
	if inv.RegimeDef() == nil {
		return nil, fmt.Errorf("invoice requires a tax regime (usually derived from the supplier's tax ID)")
	}
	base, err := ubl.ConvertInvoice(env, ubl.WithContext(ContextOIOUBL))
	if err != nil {
		return nil, err
	}
	out := (*Invoice)(base)
	if err := out.applyOIOUBLFlavor(inv); err != nil {
		return nil, err
	}
	return out, nil
}

func Bytes(in any) ([]byte, error) {
	return ubl.Bytes(in)
}

func BytesCompact(in any) ([]byte, error) {
	return ubl.BytesCompact(in)
}
