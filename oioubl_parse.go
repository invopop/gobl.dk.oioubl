package oioubl

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"slices"
	"strings"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl.dk.oioubl/addon"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
)

const (
	rootNameCreditNote = "CreditNote"
)

// supportedCustomizationIDs are the OIOUBL 2.1 profile ids in use. A prefix test
// would also pass unrelated ids straight into the destructive strip pass.
var supportedCustomizationIDs = []string{"OIOUBL-2.01", "OIOUBL-2.02", "OIOUBL-2.1"}

// BinaryAttachment is the base's type, re-exported so callers do not have to
// import gobl.ubl just to read what ExtractBinaryAttachments hands back.
type BinaryAttachment = ubl.BinaryAttachment

// oioublDetails is what the strip pass takes out of the document, held over so
// the add pass can put it back into GOBL.
type oioublDetails struct {
	docDuties   []exciseDuty
	lineDuties  map[int][]exciseDuty
	vatPercents map[string]string
}

// Parse reads an OIOUBL document off the wire.
func Parse(data []byte) (any, error) {
	doc, err := ubl.Parse(data)
	if err != nil {
		return nil, err
	}
	in, ok := doc.(*ubl.Invoice)
	if !ok {
		return nil, ubl.ErrUnknownDocumentType
	}
	return (*Invoice)(in), nil
}

// ParseInvoice is Parse for callers that already know it is an invoice.
func ParseInvoice(data []byte) (*Invoice, error) {
	doc, err := Parse(data)
	if err != nil {
		return nil, err
	}
	inv, ok := doc.(*Invoice)
	if !ok {
		return nil, fmt.Errorf("expected invoice, got %T", doc)
	}
	return inv, nil
}

// ExtractBinaryAttachments returns the files embedded in the document. Invoice
// is its own type, so the base's method does not come along and is forwarded.
func (ui *Invoice) ExtractBinaryAttachments() []BinaryAttachment {
	return (*ubl.Invoice)(ui).ExtractBinaryAttachments()
}

// Convert strips the document back to plain EN16931, runs the generic parse,
// then adds the OIOUBL details the base has no field for.
func (ui *Invoice) Convert() (*gobl.Envelope, error) {
	if !slices.Contains(supportedCustomizationIDs, ui.CustomizationID) {
		return nil, fmt.Errorf("unsupported customization id %q, expected one of %v", ui.CustomizationID, supportedCustomizationIDs)
	}

	// Stripping rewrites the document, so work on a copy: the caller keeps a
	// usable document and a second Convert does not see a half-stripped one.
	work, err := ui.clone()
	if err != nil {
		return nil, err
	}

	details, err := work.stripOIOUBL()
	if err != nil {
		return nil, err
	}

	env, err := (*ubl.Invoice)(work).Convert(ubl.WithContext(ubl.ContextEN16931))
	if err != nil {
		return nil, err
	}
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return nil, ErrUnsupportedDocumentType
	}

	work.addGOBLDetails(inv, details)

	inv.SetAddons(append(inv.GetAddons(), addon.V2)...)
	if err := env.Calculate(); err != nil {
		return nil, err
	}
	if err := env.Validate(); err != nil {
		return nil, err
	}
	return env, nil
}

// stripDocumentTypes lowercases OIOUBL's CamelCase cbc:DocumentType, which the
// generic parser casts straight into a GOBL key and which no key pattern allows.
func (ui *Invoice) stripDocumentTypes() {
	for _, refs := range [][]ubl.Reference{ui.ContractDocumentReference, ui.AdditionalDocumentReference} {
		for i := range refs {
			refs[i].DocumentType = strings.ToLower(refs[i].DocumentType)
		}
	}
}

// clone deep-copies the document so stripping cannot reach the caller's own.
func (ui *Invoice) clone() (*Invoice, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode((*ubl.Invoice)(ui)); err != nil {
		return nil, err
	}
	out := new(ubl.Invoice)
	if err := gob.NewDecoder(&buf).Decode(out); err != nil {
		return nil, err
	}
	return (*Invoice)(out), nil
}

// stripOIOUBL rewrites the document in place into something the generic parser
// reads correctly, handing back the excise duties it has no field for.
func (ui *Invoice) stripOIOUBL() (oioublDetails, error) {
	ui.stripParties()
	ui.stripDocumentTypes()
	ui.stripPaymentDueDate()
	ui.stripDelivery()

	details := oioublDetails{vatPercents: make(map[string]string)}

	lineDuties, err := ui.stripLines(details.vatPercents)
	if err != nil {
		return details, err
	}
	details.lineDuties = lineDuties

	ui.TaxTotal, details.docDuties, err = splitExciseTaxTotals(ui.TaxTotal)
	if err != nil {
		return details, err
	}
	collectVATPercents(ui.TaxTotal, details.vatPercents)
	for i := range ui.TaxTotal {
		stripTaxTotalCategories(&ui.TaxTotal[i])
	}
	for i := range ui.AllowanceCharge {
		for _, tc := range ui.AllowanceCharge[i].TaxCategory {
			stripTaxCategory(tc)
		}
		stripAllowanceMultiplier(&ui.AllowanceCharge[i])
	}

	return details, nil
}
