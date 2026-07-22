package dkoioubl

import (
	"github.com/invopop/gobl"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/invopop/gobl/uuid"
)

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

// OIOUBL: also parses document-level excise duties via goblAddExciseCharges.
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

	// The regime normally derives from the supplier's tax ID country, but a
	// CPR-only (non-VAT-registered) supplier has no tax ID; an OIOUBL document
	// is Danish by definition, and a missing regime breaks the build direction.
	if out.Supplier == nil || out.Supplier.TaxID == nil || out.Supplier.TaxID.Country == "" {
		out.SetRegime("DK")
	}

	if u, err := uuid.Parse(ui.UUID); err == nil {
		out.UUID = u
	}

	ui.resolveInvoiceType(out)

	if err := (*ubl.Invoice)(ui).ParseInvoiceDates(out); err != nil {
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
	if err := ui.applyOrderingExtras(out); err != nil {
		return nil, err
	}
	if err := ui.goblAddDelivery(out); err != nil {
		return nil, err
	}

	(*ubl.Invoice)(ui).ParseInvoiceNotes(out)

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

// OIOUBL: a credit note omits the type code, so the root element decides the type.
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
	out.Type = ubl.TypeCodeParse(typeCode)
	if tags := ubl.TagCodeParse(typeCode, ubl.Context{}); len(tags) != 0 {
		out.SetTags(tags...)
	}
}

// applyOrderingExtras reads the ordering fields the shared GoblAddOrdering
// doesn't: the document-level cbc:AccountingCost (mirroring the outbound side,
// which emits it from Ordering.Cost) and the OrderReference issue date.
func (ui *Invoice) applyOrderingExtras(out *bill.Invoice) error {
	if ui.AccountingCost != "" {
		if out.Ordering == nil {
			out.Ordering = new(bill.Ordering)
		}
		out.Ordering.Cost = cbc.Code(ui.AccountingCost)
	}
	if ui.OrderReference != nil && ui.OrderReference.IssueDate != "" &&
		out.Ordering != nil && len(out.Ordering.Purchases) > 0 {
		d, err := ubl.ParseDate(ui.OrderReference.IssueDate)
		if err != nil {
			return err
		}
		out.Ordering.Purchases[0].IssueDate = &d
	}
	return nil
}

func (ui *Invoice) applyExchangeRates(out *bill.Invoice) {
	if ui.TaxCurrencyCode == "" || ui.DocumentCurrencyCode == ui.TaxCurrencyCode {
		return
	}
	docCurrency := currency.Code(ui.DocumentCurrencyCode)
	taxCurrency := currency.Code(ui.TaxCurrencyCode)

	out.ExchangeRates = ubl.GoblExchangeRates(docCurrency, taxCurrency, ui.TaxTotal)
	if out.ExchangeRates == nil {
		// OIOUBL: the tax-currency amount is always carried inside the single
		// TaxTotal block via each subtotal's TransactionCurrencyTaxAmount
		// (F-INV018/F-CRN013), never as a second TaxTotal block.
		out.ExchangeRates = exchangeRatesFromTransactionCurrency(docCurrency, taxCurrency, ui.TaxTotal)
	}
}

func exchangeRatesFromTransactionCurrency(docCurrency, taxCurrency currency.Code, taxTotals []ubl.TaxTotal) []*currency.ExchangeRate {
	if len(taxTotals) != 1 {
		return nil
	}

	docAmount, err := num.AmountFromString(ubl.NormalizeNumericString(taxTotals[0].TaxAmount.Value))
	if err != nil || docAmount.IsZero() {
		return nil
	}

	var total num.Amount
	found := false
	for _, st := range taxTotals[0].TaxSubtotal {
		if st.TransactionCurrencyTaxAmount == nil {
			continue
		}
		a, err := num.AmountFromString(ubl.NormalizeNumericString(st.TransactionCurrencyTaxAmount.Value))
		if err != nil {
			return nil
		}
		if found {
			total = total.Add(a)
		} else {
			total, found = a, true
		}
	}
	if !found {
		return nil
	}

	rate := total.Divide(docAmount)
	return []*currency.ExchangeRate{
		{
			From:   docCurrency,
			To:     taxCurrency,
			Amount: rate,
		},
	}
}

func (ui *Invoice) parseBillingReferences(out *bill.Invoice) error {
	if len(ui.BillingReference) == 0 {
		return nil
	}
	out.Preceding = make([]*org.DocumentRef, 0, len(ui.BillingReference))
	for _, ref := range ui.BillingReference {
		var src *Reference
		switch {
		case ref.InvoiceDocumentReference != nil:
			src = ref.InvoiceDocumentReference
		case ref.SelfBilledInvoiceDocumentReference != nil:
			src = ref.SelfBilledInvoiceDocumentReference
		case ref.CreditNoteDocumentReference != nil:
			src = ref.CreditNoteDocumentReference
		case ref.AdditionalDocumentReference != nil:
			src = ref.AdditionalDocumentReference
		default:
			continue
		}
		docRef, err := ubl.GoblReference(src)
		if err != nil {
			return err
		}
		if docRef == nil {
			continue
		}
		// The shared GoblReference drops the referenced document's UUID.
		if u, err := uuid.Parse(src.UUID); err == nil {
			docRef.UUID = u
		}
		out.Preceding = append(out.Preceding, docRef)
	}
	return nil
}

