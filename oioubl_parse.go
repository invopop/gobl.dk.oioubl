package oioubl

import (
	"fmt"
	"strings"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl.dk.oioubl/addon"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
)

const rootNameCreditNote = "CreditNote"

// Matches "OIOUBL-2.1", "OIOUBL-2.01", "OIOUBL-2.02", etc.
const customizationIDPrefix = "OIOUBL-2"

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
func (ui *Invoice) ExtractBinaryAttachments() []ubl.BinaryAttachment {
	return (*ubl.Invoice)(ui).ExtractBinaryAttachments()
}

// Convert strips the document back to plain EN16931, runs the generic parse,
// then adds the OIOUBL details the base has no field for.
func (ui *Invoice) Convert() (*gobl.Envelope, error) {
	if !strings.HasPrefix(ui.CustomizationID, customizationIDPrefix) {
		return nil, fmt.Errorf("unsupported customization id %q, expected an %q document", ui.CustomizationID, customizationIDPrefix)
	}

	docExcises, lineExcises, vatPercents, err := ui.stripOIOUBL()
	if err != nil {
		return nil, err
	}

	env, err := (*ubl.Invoice)(ui).Convert(ubl.WithContext(ubl.ContextEN16931))
	if err != nil {
		return nil, err
	}
	inv, ok := env.Extract().(*bill.Invoice)
	if !ok {
		return nil, ErrUnsupportedDocumentType
	}

	ui.addOIOUBLDetails(inv, docExcises, lineExcises, vatPercents)

	inv.SetAddons(append(inv.GetAddons(), addon.V2)...)
	if err := env.Calculate(); err != nil {
		return nil, err
	}
	if err := env.Validate(); err != nil {
		return nil, err
	}
	return env, nil
}

// stripOIOUBL rewrites the document in place into something the generic parser
// reads correctly, handing back the excise duties it has no field for.
func (ui *Invoice) stripOIOUBL() (docExcises []exciseDuty, lineExcises map[int][]exciseDuty, vatPercents map[string]string, err error) {
	ui.stripParties()
	ui.stripPaymentDueDate()
	ui.stripDelivery()

	vatPercents = make(map[string]string)

	lineExcises, err = ui.stripLines(vatPercents)
	if err != nil {
		return nil, nil, nil, err
	}

	ui.TaxTotal, docExcises, err = splitExciseTaxTotals(ui.TaxTotal)
	if err != nil {
		return nil, nil, nil, err
	}
	collectVATPercents(ui.TaxTotal, vatPercents)
	for i := range ui.TaxTotal {
		stripTaxTotalCategories(&ui.TaxTotal[i])
	}
	for i := range ui.AllowanceCharge {
		for _, tc := range ui.AllowanceCharge[i].TaxCategory {
			stripTaxCategoryWire(tc)
		}
	}

	return docExcises, lineExcises, vatPercents, nil
}
