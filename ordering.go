package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/org"
)

// Identical to gobl.ubl.
func (ui *Invoice) addPreceding(refs []*org.DocumentRef) {
	if len(refs) == 0 {
		return
	}
	ui.BillingReference = make([]*BillingReference, len(refs))
	for i, ref := range refs {
		r := &Reference{
			ID: IDType{Value: ref.Series.Join(ref.Code).String()},
		}
		if ref.IssueDate != nil {
			r.IssueDate = ref.IssueDate.String()
		}
		if dt := ref.Ext.Get(untdid.ExtKeyDocumentType); dt != "" {
			r.DocumentTypeCode = dt.String()
		}
		ui.BillingReference[i] = &BillingReference{
			InvoiceDocumentReference: r,
		}
	}
}

// Identical to gobl.ubl.
func (ui *Invoice) addOrdering(o *bill.Ordering) {
	if o != nil {
		if o.Code != "" {
			ui.BuyerReference = o.Code.String()
		}

		// If both ordering.seller and seller are present, the original seller is used
		// as the tax representative.
		if o.Seller != nil {
			p := ui.AccountingSupplierParty.Party
			ui.TaxRepresentativeParty = p
			ui.AccountingSupplierParty = SupplierParty{
				Party: newParty(o.Seller),
			}
		}

		if o.Period != nil {
			ui.InvoicePeriod = []Period{
				{
					StartDate: ubl.FormatDate(o.Period.Start),
					EndDate:   ubl.FormatDate(o.Period.End),
				},
			}
		}

		if len(o.Purchases) > 0 {
			purchase := o.Purchases[0]
			ui.OrderReference = &OrderReference{
				ID: purchase.Code.String(),
			}
		}

		// BT-14: Sales order reference
		if len(o.Sales) > 0 {
			if ui.OrderReference == nil {
				ui.OrderReference = &OrderReference{
					ID: "NA",
				}
			}
			ui.OrderReference.SalesOrderID = o.Sales[0].Code.String()
		}

		// BT-11: Project reference
		for _, proj := range o.Projects {
			ui.ProjectReference = append(ui.ProjectReference, ProjectReference{
				ID: proj.Code.String(),
			})
		}

		for _, despatch := range o.Despatch {
			ui.DespatchDocumentReference = append(ui.DespatchDocumentReference, Reference{
				ID: IDType{Value: string(despatch.Code)},
			})
		}

		for _, receiving := range o.Receiving {
			ui.ReceiptDocumentReference = append(ui.ReceiptDocumentReference, Reference{
				ID: IDType{Value: string(receiving.Code)},
			})
		}

		for _, contract := range o.Contracts {
			ui.ContractDocumentReference = append(ui.ContractDocumentReference, Reference{
				ID: IDType{Value: string(contract.Code)},
			})
		}

		for _, tender := range o.Tender {
			ui.OriginatorDocumentReference = append(ui.OriginatorDocumentReference, Reference{
				ID: IDType{Value: string(tender.Code)},
			})
		}

		if len(o.Identities) > 0 {
			ioi := o.Identities[0]

			for _, id := range o.Identities {
				if id.Ext.Has(untdid.ExtKeyReference) {
					ioi = id
					break
				}
			}

			id := IDType{Value: string(ioi.Code)}
			if ref := ioi.Ext.Get(untdid.ExtKeyReference); ref != "" {
				schemeID := ref.String()
				id.SchemeID = &schemeID
			}
			ui.AdditionalDocumentReference = append(ui.AdditionalDocumentReference, Reference{
				ID:               id,
				DocumentTypeCode: "130",
			})
		}
	}

	// BT-13: Ensure at least one of BuyerReference or OrderReference is set.
	if ui.BuyerReference == "" && (ui.OrderReference == nil || ui.OrderReference.ID == "") {
		if ui.OrderReference == nil {
			ui.OrderReference = &OrderReference{}
		}
		ui.OrderReference.ID = "NA"
	}
}
