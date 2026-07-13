package dkoioubl

import (
	"cloud.google.com/go/civil"
	"github.com/invopop/gobl"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

var invoiceTypeMap = map[string]cbc.Key{
	"325": bill.InvoiceTypeProforma,
	"380": bill.InvoiceTypeStandard,
	"381": bill.InvoiceTypeCreditNote,
	"383": bill.InvoiceTypeDebitNote,
	"384": bill.InvoiceTypeCorrective,
	"388": bill.InvoiceTypeStandard,
	"389": bill.InvoiceTypeStandard,
	"326": bill.InvoiceTypeStandard,
	"261": bill.InvoiceTypeCreditNote,
}

// InvoiceTagMap maps UBL invoice type codes to GOBL tax tags.
var InvoiceTagMap = map[string][]cbc.Key{
	"389": {tax.TagSelfBilled},
	"326": {tax.TagPartial},
	"261": {tax.TagSelfBilled},
}

// Convert converts the OIOUBL Invoice to a GOBL envelope. Binary attachments are
// ignored here — use ExtractBinaryAttachments to retrieve them separately.
func (ui *Invoice) Convert() (*gobl.Envelope, error) {
	inv, err := ui.goblInvoice()
	if err != nil {
		return nil, err
	}

	env := gobl.NewEnvelope()
	if err := env.Insert(inv); err != nil {
		return nil, err
	}

	return env, nil
}

func (ui *Invoice) goblInvoice() (*bill.Invoice, error) {
	out := &bill.Invoice{
		Addons:   tax.Addons{List: Addons},
		Code:     cbc.Code(ui.ID),
		Currency: currency.Code(ui.DocumentCurrencyCode),
		Tax: &bill.Tax{
			// Currency rounding is the EN16931 default for incoming invoices.
			Rounding: tax.RoundingRuleCurrency,
		},
		Supplier: goblParty(ui.AccountingSupplierParty.Party),
		Customer: goblParty(ui.AccountingCustomerParty.Party),
	}

	ui.resolveInvoiceType(out)

	if err := ui.parseInvoiceDates(out); err != nil {
		return nil, err
	}
	ui.applyExchangeRates(out)

	if err := ui.goblAddLines(out); err != nil {
		return nil, err
	}
	if err := ui.goblAddPayment(out); err != nil {
		return nil, err
	}
	if err := (*ubl.Invoice)(ui).GoblAddOrdering(out); err != nil {
		return nil, err
	}
	if err := ui.goblAddDelivery(out); err != nil {
		return nil, err
	}

	ui.parseInvoiceNotes(out)

	if err := ui.parseBillingReferences(out); err != nil {
		return nil, err
	}
	ui.applyTaxRepresentative(out)

	if len(ui.AllowanceCharge) > 0 {
		if err := ui.goblAddCharges(out); err != nil {
			return nil, err
		}
	}
	if err := ui.goblAddExciseCharges(out); err != nil {
		return nil, err
	}

	out.Attachments = (*ubl.Invoice)(ui).GoblAddAttachments()
	ui.goblAddTaxNotes(out)

	return out, nil
}

func (ui *Invoice) resolveInvoiceType(out *bill.Invoice) {
	typeCode := ui.InvoiceTypeCode
	if typeCode == nil {
		typeCode = ui.CreditNoteTypeCode
	}
	if typeCode == nil && ui.XMLName.Local == rootNameCreditNote {
		// OIOUBL omits the credit-note type code; the root element is authoritative.
		out.Type = bill.InvoiceTypeCreditNote
		return
	}
	out.Type = typeCodeParse(typeCode)
	if tags := tagCodeParse(typeCode); len(tags) != 0 {
		out.SetTags(tags...)
	}
}

func (ui *Invoice) parseInvoiceDates(out *bill.Invoice) error {
	issueDate, err := ubl.ParseDate(ui.IssueDate)
	if err != nil {
		return err
	}
	out.IssueDate = issueDate

	if ui.IssueTime != "" {
		ct, err := civil.ParseTime(ui.IssueTime)
		if err != nil {
			return err
		}
		out.IssueTime = &cal.Time{Time: ct}
	}

	// BT-7: VAT point date
	if ui.TaxPointDate != "" {
		vd, err := ubl.ParseDate(ui.TaxPointDate)
		if err != nil {
			return err
		}
		out.ValueDate = &vd
	}

	return nil
}

// Identical to gobl.ubl.applyExchangeRates.
func (ui *Invoice) applyExchangeRates(out *bill.Invoice) {
	if ui.TaxCurrencyCode != "" && ui.DocumentCurrencyCode != ui.TaxCurrencyCode {
		out.ExchangeRates = goblExchangeRates(
			currency.Code(ui.DocumentCurrencyCode),
			currency.Code(ui.TaxCurrencyCode),
			ui.TaxTotal,
		)
	}
}

func (ui *Invoice) parseInvoiceNotes(out *bill.Invoice) {
	if len(ui.Note) == 0 {
		return
	}
	out.Notes = make([]*org.Note, 0, len(ui.Note))
	for _, note := range ui.Note {
		out.Notes = append(out.Notes, ubl.ParseNote(note))
	}
}

func (ui *Invoice) parseBillingReferences(out *bill.Invoice) error {
	if len(ui.BillingReference) == 0 {
		return nil
	}
	out.Preceding = make([]*org.DocumentRef, 0, len(ui.BillingReference))
	for _, ref := range ui.BillingReference {
		var (
			docRef *org.DocumentRef
			err    error
		)
		switch {
		case ref.InvoiceDocumentReference != nil:
			docRef, err = ubl.GoblReference(ref.InvoiceDocumentReference)
		case ref.SelfBilledInvoiceDocumentReference != nil:
			docRef, err = ubl.GoblReference(ref.SelfBilledInvoiceDocumentReference)
		case ref.CreditNoteDocumentReference != nil:
			docRef, err = ubl.GoblReference(ref.CreditNoteDocumentReference)
		case ref.AdditionalDocumentReference != nil:
			docRef, err = ubl.GoblReference(ref.AdditionalDocumentReference)
		}
		if err != nil {
			return err
		}
		if docRef != nil {
			out.Preceding = append(out.Preceding, docRef)
		}
	}
	return nil
}

func (ui *Invoice) applyTaxRepresentative(out *bill.Invoice) {
	if ui.TaxRepresentativeParty == nil {
		return
	}
	if out.Ordering == nil {
		out.Ordering = &bill.Ordering{}
	}
	out.Ordering.Seller = out.Supplier

	out.Supplier = goblParty(ui.TaxRepresentativeParty)
}

// Identical to gobl.ubl.typeCodeParse.
func typeCodeParse(typeCode *IDType) cbc.Key {
	if typeCode == nil {
		return bill.InvoiceTypeOther
	}
	if val, ok := invoiceTypeMap[typeCode.Value]; ok {
		return val
	}
	return bill.InvoiceTypeOther
}

func tagCodeParse(typeCode *IDType) []cbc.Key {
	if typeCode == nil {
		return nil
	}
	return InvoiceTagMap[typeCode.Value]
}
