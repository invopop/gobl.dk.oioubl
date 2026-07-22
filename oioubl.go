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

// Invoice is the OIOUBL view of a UBL invoice: a defined type over ubl.Invoice
// (not an alias) so the OIOUBL build/parse methods hang off it and gobl.ubl's
// generic Convert is not inherited. It shares the wire layout, so it marshals identically.
type Invoice ubl.Invoice

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

// convertViaOverlay builds gobl.ubl's plain EN16931 base document, then
// applies the OIOUBL flavor on top -- the base builder never sees OIOUBL's
// own context, so it stays entirely decoupled from OIOUBL specifics.
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
	// The EN16931 base only ensures its own addon; the flavor reads data the
	// OIOUBL addon normalizes in (legal identities, endpoints), so apply it here.
	if err := ensureOIOUBLAddon(env, inv); err != nil {
		return nil, err
	}
	base, err := ubl.ConvertInvoice(env, ubl.WithContext(ubl.ContextEN16931))
	if err != nil {
		return nil, err
	}
	out := (*Invoice)(base)
	if err := out.applyOIOUBLFlavor(inv); err != nil {
		return nil, err
	}
	return out, nil
}

// ensureOIOUBLAddon declares the OIOUBL addon on the invoice if absent and
// recalculates the envelope so its normalizations and validations apply.
func ensureOIOUBLAddon(env *gobl.Envelope, inv *bill.Invoice) error {
	if oioubl.V2.In(inv.GetAddons()...) {
		return nil
	}
	inv.SetAddons(append(inv.GetAddons(), oioubl.V2)...)
	if err := env.Calculate(); err != nil {
		return err
	}
	return env.Validate()
}

func Bytes(in any) ([]byte, error) {
	return ubl.Bytes(in)
}

func BytesCompact(in any) ([]byte, error) {
	return ubl.BytesCompact(in)
}
