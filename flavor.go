package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"

	"github.com/invopop/gobl/bill"
)

// applyOIOUBLFlavor turns gobl.ubl's EN 16931 base document into an OIOUBL 2.1
// one. gobl.ubl already builds the header, ordering (including the
// seller/tax-representative swap), preceding references, notes, attachments,
// parties, and the ordinary (non-excise, non-Giro/FIK) parts of payment —
// those subtrees are decorated in place rather than rebuilt. Charges, totals,
// and lines use OIOUBL-specific business rules (gross line amounts, excise
// duties as their own tax category, per-line rounding) with no equivalent in
// the generic base, so those are still fully replaced.
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

	// Reset contract: charges/totals/lines have no reusable equivalent in the
	// base (gross vs net amounts, excise-as-tax, per-line rounding), so they're
	// still replaced outright. A gobl.ubl bump that starts populating these
	// differently would double-emit, so bump reviews must re-audit this list.
	ui.AllowanceCharge = nil
	ui.TaxTotal = nil
	ui.addCharges(inv)
	ui.addTotals(inv)
	ui.InvoiceLines = nil
	ui.CreditNoteLines = nil
	ui.addLines(inv)

	// Payment: the base's AddPayment already built the ordinary (bank
	// transfer/direct debit/card) case; decorate it in place for OIOUBL's
	// channel code and BIC nesting, and only replace it outright for Giro/FIK,
	// which have no equivalent shape in the base (CreditAccount + kortart).
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

// decorateParties adjusts the base's already-correct supplier/customer/
// tax-representative/payee parties (built by gobl.ubl's NewParty, including
// the ordering.seller tax-representative swap) with OIOUBL's extras and
// scheme stamping, instead of rebuilding them from scratch.
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

// ContextOIOUBL drives gobl.ubl's generic builder with OIOUBL's document
// identifiers and addon. Only the dk-oioubl addon is listed; it requires
// en16931, so the base is calculated and validated exactly as before.
var ContextOIOUBL = ubl.Context{
	CustomizationID: CustomizationID,
	ProfileID:       ProfileID,
	Addons:          Addons,
	VESIDs: ubl.VESIDMapping{
		Invoice:    VESIDInvoice,
		CreditNote: VESIDCreditNote,
	},
}
