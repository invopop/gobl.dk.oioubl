package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"

	"github.com/invopop/gobl/bill"
)

// applyOIOUBLFlavor turns gobl.ubl's EN 16931 base document into an OIOUBL 2.1
// one: gobl.ubl builds the shared UBL scaffold (header, ordering, preceding,
// notes, attachments), and the OIOUBL-specific subtrees — parties, monetary
// totals, lines, payment, delivery — are rebuilt from the GOBL invoice and the
// OIOUBL attributes stamped on top.
func (ui *Invoice) applyOIOUBLFlavor(inv *bill.Invoice) error {
	ui.UBLVersionID = Version
	stampProfileID(ui.ProfileID)
	if !inv.UUID.IsZero() {
		ui.UUID = inv.UUID.String()
	}
	if inv.Ordering != nil && inv.Ordering.Cost != "" {
		ui.AccountingCost = inv.Ordering.Cost.String()
	}
	// BR-53: the restated tax rides StandardRated subtotals only, so the tax
	// currency is dropped when none is present (F-LIB373 / F-INV018).
	if ui.TaxCurrencyCode != "" && !hasStandardRated(inv) {
		ui.TaxCurrencyCode = ""
	}

	// Reset contract: every base subtree reset to nil below is rebuilt from the
	// GOBL invoice; everything else is kept verbatim from gobl.ubl. A gobl.ubl
	// bump that starts populating one of these fields would double-emit, so
	// bump reviews must re-audit this list.
	ui.rebuildParties(inv)

	ui.AllowanceCharge = nil
	ui.TaxTotal = nil
	ui.PrepaidPayment = nil
	ui.addCharges(inv)
	ui.addTotals(inv)
	ui.InvoiceLines = nil
	ui.CreditNoteLines = nil
	ui.addLines(inv)

	ui.PaymentMeans = nil
	ui.PayeeParty = nil
	ui.PaymentTerms = nil
	if err := ui.addPayment(inv); err != nil {
		return err
	}

	if d := newDelivery(inv.Delivery); d != nil {
		ui.Delivery = []*Delivery{d}
	} else {
		ui.Delivery = nil
	}

	applyTypeCode(ui.InvoiceTypeCode)
	applyTypeCode(ui.CreditNoteTypeCode)
	applyParty(ui.AccountingSupplierParty.Party)
	applyParty(ui.AccountingCustomerParty.Party)
	applyParty(ui.PayeeParty)
	applyTaxRepParty(ui.TaxRepresentativeParty)
	ui.applyBillingReference()
	ui.applyAttachments()
	ui.applyTotals()
	return nil
}

// rebuildParties replaces the base's EN 16931 supplier and customer parties with
// OIOUBL ones and reapplies the tax-representative swap: when the ordering
// carries a seller, the invoice supplier becomes the tax representative and the
// ordering seller becomes the accounting supplier.
func (ui *Invoice) rebuildParties(inv *bill.Invoice) {
	ui.AccountingSupplierParty = SupplierParty{Party: newParty(inv.Supplier)}
	ui.AccountingCustomerParty = CustomerParty{Party: newParty(inv.Customer)}
	ui.TaxRepresentativeParty = nil
	if inv.Ordering != nil && inv.Ordering.Seller != nil {
		ui.TaxRepresentativeParty = ui.AccountingSupplierParty.Party
		ui.AccountingSupplierParty = SupplierParty{Party: newParty(inv.Ordering.Seller)}
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
