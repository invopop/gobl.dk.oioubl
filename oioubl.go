// Package dkoioubl converts GOBL documents to OIOUBL 2.1, the Danish
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

// Invoice is the OIOUBL view of a UBL invoice: a defined type over ubl.Invoice, not an
// alias, so OIOUBL's own methods attach to it without inheriting gobl.ubl's generic Convert.
type Invoice ubl.Invoice

const Version = ubl.Version

const (
	CustomizationID = "OIOUBL-2.1"
	ProfileID       = "urn:www.nesubl.eu:profiles:profile5:ver2.0"
)

const (
	VESIDInvoice    = "dk.oioubl:invoice:1.17.2"
	VESIDCreditNote = "dk.oioubl:credit-note:1.17.2"
)

// OIOUBL code-list and scheme identifiers the schematron expects (agency 320 throughout).
const (
	agencyID = "320"

	schemeTaxCategory = "urn:oioubl:id:taxcategoryid-1.1"
	schemeTaxScheme   = "urn:oioubl:id:taxschemeid-1.5"
	schemeProfileV12  = "urn:oioubl:id:profileid-1.2"

	codelistInvoiceType    = "urn:oioubl:codelist:invoicetypecode-1.1"
	codelistPaymentChannel = "urn:oioubl:codelist:paymentchannelcode-1.1"
	codelistAddressFormat  = "urn:oioubl:codelist:addressformatcode-1.1"
	codelistTaxType        = "urn:oioubl:codelist:taxtypecode-1.1"
)

var ErrUnsupportedDocumentType = fmt.Errorf("unsupported document type")

var Addons = []cbc.Key{oioubl.V2}

// Convert turns a GOBL envelope into an OIOUBL 2.1 document.
func Convert(env *gobl.Envelope) (any, error) {
	out, err := buildInvoice(env)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ConvertInvoice is Convert for callers that already know the document is an invoice.
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

// Bytes renders a converted document as indented XML.
func Bytes(in any) ([]byte, error) {
	return ubl.Bytes(in)
}

// BytesCompact renders a converted document as XML without indentation.
func BytesCompact(in any) ([]byte, error) {
	return ubl.BytesCompact(in)
}

// GetVESID names the schematron to validate a document against.
func GetVESID(inv *bill.Invoice) string {
	if inv.Type.In(bill.InvoiceTypeCreditNote) {
		return VESIDCreditNote
	}
	return VESIDInvoice
}

// buildInvoice builds the plain EN16931 document, then reworks it into OIOUBL.
func buildInvoice(env *gobl.Envelope) (*Invoice, error) {
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return nil, ErrUnsupportedDocumentType
	}
	// The base only ensures its own addon, and the steps below read what the
	// OIOUBL addon adds (legal identities, endpoints), so apply it first.
	if err := ensureOIOUBLAddon(env, inv); err != nil {
		return nil, err
	}
	base, err := ubl.ConvertInvoice(env, ubl.WithContext(ubl.ContextEN16931))
	if err != nil {
		return nil, err
	}
	out := (*Invoice)(base)
	out.applyOIOUBL(inv)
	return out, nil
}

// applyOIOUBL reworks the EN16931 document into OIOUBL 2.1. The totals have no
// reusable equivalent in the base, so they are rebuilt from scratch.
func (ui *Invoice) applyOIOUBL(inv *bill.Invoice) {
	ui.applyParties(inv)

	// Drop the tax currency unless a StandardRated rate (%>0) carries it (F-LIB373/F-INV018).
	if ui.TaxCurrencyCode != "" && !hasStandardRated(inv) {
		ui.TaxCurrencyCode = ""
	}
	ui.applyCharges(inv)
	ui.TaxTotal = nil
	ui.addTotals(inv)
	ui.applyLines(inv)
	ui.applyTotals()

	ui.applySchemes(inv)
}

// ensureOIOUBLAddon adds the addon and recalculates, so its normalizations run.
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
