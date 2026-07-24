package dkoioubl

import (
	"fmt"
	"strings"

	"github.com/invopop/gobl"
	oioubl "github.com/invopop/gobl.dk.oioubl/addon"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
)

// Strips the Unicode replacement character (U+FFFD), which breaks canonical
// JSON serialization if left in.
func cleanString(s string) string {
	return strings.ReplaceAll(s, "�", "")
}

const rootNameCreditNote = "CreditNote"

// Matches "OIOUBL-2.1", "OIOUBL-2.01", "OIOUBL-2.02", etc.
const customizationIDPrefix = "OIOUBL-2"

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

// Strip to plain EN16931, run the generic parse, then decorate with the
// OIOUBL specifics the base has no field for (excise, DK payment shapes).
func (ui *Invoice) Convert() (*gobl.Envelope, error) {
	if !strings.HasPrefix(ui.CustomizationID, customizationIDPrefix) {
		return nil, fmt.Errorf("unsupported customization id %q, expected an %q document", ui.CustomizationID, customizationIDPrefix)
	}

	docExcises, lineExcises, vatPercents, err := ui.stripOIOUBLFlavor()
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

	ui.decorateGOBL(inv, docExcises, lineExcises, vatPercents)

	inv.SetAddons(append(inv.GetAddons(), oioubl.V2)...)
	if err := env.Calculate(); err != nil {
		return nil, err
	}
	if err := env.Validate(); err != nil {
		return nil, err
	}
	return env, nil
}

// Mutates the wire document in place, undoing OIOUBL decorations the generic
// parser would misread, and returns excise duties extracted along the way.
func (ui *Invoice) stripOIOUBLFlavor() (docExcises []exciseDuty, lineExcises map[int][]exciseDuty, vatPercents map[string]string, err error) {
	ui.stripPartyFlavor()
	ui.stripPaymentDueDate()
	ui.stripDeliveryFlavor()

	vatPercents = make(map[string]string)

	lineExcises, err = ui.stripLineFlavor(vatPercents)
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
