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
	"github.com/invopop/gobl/cal"
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

func GetVESID(inv *bill.Invoice) string {
	if inv.Type.In(bill.InvoiceTypeCreditNote) {
		return VESIDCreditNote
	}
	return VESIDInvoice
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

// convertViaOverlay builds the plain EN16931 base, then decorates it into OIOUBL.
func convertViaOverlay(env *gobl.Envelope) (*Invoice, error) {
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return nil, ErrUnsupportedDocumentType
	}
	// totals.go dereferences RegimeDef() unconditionally; a regime-less
	// invoice (e.g. an unsupported supplier country) would panic there.
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
	err = out.applyOIOUBLFlavor(inv)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// applyOIOUBLFlavor turns gobl.ubl's plain EN16931 base into OIOUBL 2.1, in
// three stages: party/address, then line/categories/tax_total, then schemes.
func (ui *Invoice) applyOIOUBLFlavor(inv *bill.Invoice) error {
	ui.applyPartyAndAddressFlavor(inv)
	if err := ui.applyLineCategoryAndTaxTotalFlavor(inv); err != nil {
		return err
	}
	ui.applySchemeFlavor(inv)
	return nil
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

// formatDate renders a GOBL date in UBL's YYYY-MM-DD form.
func formatDate(d cal.Date) string {
	if d.IsZero() {
		return ""
	}
	return d.Time().Format("2006-01-02")
}

func Bytes(in any) ([]byte, error) {
	return ubl.Bytes(in)
}

func BytesCompact(in any) ([]byte, error) {
	return ubl.BytesCompact(in)
}
