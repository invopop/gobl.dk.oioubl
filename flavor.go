package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"

	"github.com/invopop/gobl/bill"
)

// applyOIOUBLFlavor turns gobl.ubl's EN 16931 base into OIOUBL 2.1: most
// subtrees are decorated in place, but charges/totals/lines are rebuilt.
func (ui *Invoice) applyOIOUBLFlavor(inv *bill.Invoice) error {
	ui.UBLVersionID = Version
	stampProfileID(ui.ProfileID)
	if !inv.UUID.IsZero() {
		ui.UUID = inv.UUID.String()
	}
	if inv.Ordering != nil && inv.Ordering.Cost != "" {
		ui.AccountingCost = inv.Ordering.Cost.String()
	}
	ui.decorateOrderingRefs(inv)
	// BR-53: the restated tax rides StandardRated subtotals only, so the tax
	// currency is dropped when none is present (F-LIB373 / F-INV018).
	if ui.TaxCurrencyCode != "" && !hasStandardRated(inv) {
		ui.TaxCurrencyCode = ""
	}

	ui.decorateParties(inv)

	// Charges/totals have no reusable equivalent in the base (excise-as-tax,
	// document-level promotion), so they're rebuilt outright.
	ui.AllowanceCharge = nil
	ui.TaxTotal = nil
	ui.addCharges(inv)
	ui.addTotals(inv)
	ui.decorateLines(inv)

	// The base already builds the ordinary payment case; decorate it for
	// OIOUBL's channel code/BIC, replacing it outright only for Giro/FIK.
	if err := ui.decoratePayment(inv); err != nil {
		return err
	}

	ui.Delivery = nil
	if d := newDelivery(inv.Delivery); d != nil {
		ui.Delivery = []*Delivery{d}
	}

	applyTypeCode(ui.InvoiceTypeCode)
	applyTypeCode(ui.CreditNoteTypeCode)
	ui.applyBillingReference()
	ui.applyAttachments()
	ui.applyTotals()
	return nil
}

// decorateParties adjusts the base's already-correct parties (built by
// gobl.ubl's NewParty) with OIOUBL's extras, instead of rebuilding them.
func (ui *Invoice) decorateParties(inv *bill.Invoice) {
	supplierSrc := inv.Supplier
	if inv.Ordering != nil && inv.Ordering.Seller != nil {
		// addOrdering already swapped AccountingSupplierParty/TaxRepresentativeParty.
		supplierSrc = inv.Ordering.Seller
		decoratePartyExtras(ui.TaxRepresentativeParty, inv.Supplier)
	}
	decoratePartyExtras(ui.AccountingSupplierParty.Party, supplierSrc)
	decoratePartyExtras(ui.AccountingCustomerParty.Party, inv.Customer)

	applyParty(ui.AccountingSupplierParty.Party)
	applyParty(ui.AccountingCustomerParty.Party)
	applyParty(ui.PayeeParty)
	applyTaxRepParty(ui.TaxRepresentativeParty)
}

// decorateOrderingRefs restores reference fields the base builder drops: the
// order reference's issue date and the preceding documents' UUIDs.
func (ui *Invoice) decorateOrderingRefs(inv *bill.Invoice) {
	if o := inv.Ordering; o != nil && len(o.Purchases) > 0 && ui.OrderReference != nil {
		if d := o.Purchases[0].IssueDate; d != nil {
			ui.OrderReference.IssueDate = ubl.FormatDate(*d)
		}
	}
	for i, ref := range inv.Preceding {
		if i >= len(ui.BillingReference) {
			break
		}
		br := ui.BillingReference[i]
		if ref.UUID.IsZero() || br == nil || br.InvoiceDocumentReference == nil {
			continue
		}
		br.InvoiceDocumentReference.UUID = ref.UUID.String()
	}
}

// stampProfileID adds the OIOUBL profileid-1.2 scheme attributes to the base's
// ProfileID value.
func stampProfileID(id *IDType) {
	if id == nil {
		return
	}
	schemeID := schemeProfileV12
	schemeAgencyID := agencyID
	id.SchemeID = &schemeID
	id.SchemeAgencyID = &schemeAgencyID
}

// ContextOIOUBL is gobl.ubl's own extension point (ubl.WithContext) for
// injecting OIOUBL's identifiers and addon into its generic builder.
var ContextOIOUBL = ubl.Context{
	CustomizationID: CustomizationID,
	ProfileID:       ProfileID,
	Addons:          Addons,
	VESIDs: ubl.VESIDMapping{
		Invoice:    VESIDInvoice,
		CreditNote: VESIDCreditNote,
	},
}
