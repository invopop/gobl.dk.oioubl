package dkoioubl

import (
	"math"
	"strings"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/cef"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

func (ui *Invoice) goblAddLines(out *bill.Invoice) error {
	items := ui.InvoiceLines
	if len(ui.CreditNoteLines) > 0 {
		items = ui.CreditNoteLines
	}

	out.Lines = make([]*bill.Line, 0, len(items))

	taxCategoryMap := ui.buildTaxCategoryMap()

	for _, docLine := range items {
		line, err := goblConvertLine(&docLine, taxCategoryMap)
		if err != nil {
			return err
		}
		if line != nil {
			out.Lines = append(out.Lines, line)
		}
	}

	return nil
}

func goblConvertLine(docLine *InvoiceLine, taxCategoryMap map[string]*taxCategoryInfo) (*bill.Line, error) {
	if docLine.Price == nil {
		return nil, nil
	}
	price, err := num.AmountFromString(ubl.NormalizeNumericString(docLine.Price.PriceAmount.Value))
	if err != nil {
		return nil, err
	}

	if docLine.Price.BaseQuantity != nil {
		// Base quantity is the number of item units to which the price applies
		baseQuantity, err := num.AmountFromString(ubl.NormalizeNumericString(docLine.Price.BaseQuantity.Value))
		if err != nil {
			return nil, err
		}
		if !baseQuantity.IsZero() {
			// Rescale to avoid rounding loss (see calculateRequiredPrecision).
			precision := calculateRequiredPrecision(price, baseQuantity)
			price = price.RescaleUp(precision).Divide(baseQuantity)
		}
	}

	line := &bill.Line{
		Quantity: num.MakeAmount(1, 0),
		Item: &org.Item{
			Price: &price,
		},
	}
	if di := docLine.Item; di != nil {
		goblConvertLineItem(di, line.Item)
		goblConvertLineItemTaxes(di, line, taxCategoryMap)
	}

	notes := make([]*org.Note, 0)

	iq := docLine.InvoicedQuantity
	if docLine.CreditedQuantity != nil {
		iq = docLine.CreditedQuantity
	}
	if iq != nil {
		line.Quantity, err = num.AmountFromString(ubl.NormalizeNumericString(iq.Value))
		if err != nil {
			return nil, err
		}

		if iq.UnitCode != "" {
			line.Item.Unit = ubl.GoblUnitFromUNECE(cbc.Code(iq.UnitCode))
		}
	}

	if len(docLine.Note) > 0 {
		for _, note := range docLine.Note {
			if note != "" {
				notes = append(notes, &org.Note{
					Text: ubl.CleanString(note),
				})
			}
		}
	}

	if docLine.AccountingCost != nil {
		// BT-133
		line.Cost = cbc.Code(*docLine.AccountingCost)
	}

	// BT-128: Invoice line object identifier
	if docLine.DocumentReference != nil && docLine.DocumentReference.ID.Value != "" {
		line.Identifier = &org.Identity{
			Code: cbc.Code(docLine.DocumentReference.ID.Value),
		}
		if docLine.DocumentReference.ID.SchemeID != nil {
			line.Identifier.Ext = tax.ExtensionsOf(cbc.CodeMap{
				untdid.ExtKeyReference: cbc.Code(*docLine.DocumentReference.ID.SchemeID),
			})
		}
	}

	if docLine.InvoicePeriod != nil {
		line.Period, err = ubl.GoblPeriodDates(docLine.InvoicePeriod)
		if err != nil {
			return nil, err
		}
	}

	if docLine.OrderLineReference != nil && docLine.OrderLineReference.LineID != "" {
		line.Order = cbc.Code(docLine.OrderLineReference.LineID)
	}

	if docLine.AllowanceCharge != nil {
		line, err = goblLineCharges(docLine.AllowanceCharge, line)
		if err != nil {
			return nil, err
		}
	}

	// OIOUBL mirrors each line's excise duties as line-level cac:TaxTotal/Excise
	// blocks; reconstruct them as line charges so the duty stays on its line.
	excise, err := exciseLineChargesFromTaxTotals(docLine.TaxTotal)
	if err != nil {
		return nil, err
	}
	line.Charges = append(line.Charges, excise...)

	if len(notes) > 0 {
		line.Notes = notes
	}
	return line, nil
}

// calculateRequiredPrecision returns the precision for a price / base-quantity
// division that avoids rounding loss: price_decimals + ceil(log10(base_quantity)).
func calculateRequiredPrecision(price, baseQuantity num.Amount) uint32 {
	priceExp := price.Exp()

	baseQtyNormalized := baseQuantity.Rescale(0)
	baseQtyFloat := math.Abs(float64(baseQtyNormalized.Value()))

	additionalDecimals := uint32(0)
	if baseQtyFloat > 1 {
		// log10(100) = 2, log10(1000) = 3, etc.
		additionalDecimals = uint32(math.Ceil(math.Log10(baseQtyFloat)))
	}

	return priceExp + additionalDecimals
}

func goblConvertLineItem(di *Item, item *org.Item) {
	if di.Name != "" {
		item.Name = ubl.CleanString(di.Name)
	}
	if di.Description != nil {
		item.Description = ubl.CleanString(*di.Description)
	}

	if di.OriginCountry != nil {
		item.Origin = l10n.ISOCountryCode(di.OriginCountry.IdentificationCode)
	}

	if di.SellersItemIdentification != nil && di.SellersItemIdentification.ID != nil {
		item.Ref = cbc.Code(di.SellersItemIdentification.ID.Value)
	}

	item.Identities = goblItemIdentities(di)

	if di.AdditionalItemProperty != nil {
		item.Meta = make(cbc.Meta)
		for _, property := range *di.AdditionalItemProperty {
			if property.Name != "" && property.Value != "" {
				key := ubl.FormatKey(property.Name)
				item.Meta[key] = ubl.CleanString(property.Value)
			}
		}
	}
}

func goblConvertLineItemTaxes(di *Item, line *bill.Line, taxCategoryMap map[string]*taxCategoryInfo) {
	ctc := di.ClassifiedTaxCategory
	if ctc == nil || ctc.TaxScheme == nil {
		return
	}

	line.Taxes = tax.Set{
		{
			Category: goblTaxSchemeCategory(ctc.TaxScheme.ID.Value),
		},
	}
	if ctc.ID != nil {
		line.Taxes[0].Ext = line.Taxes[0].Ext.Set(untdid.ExtKeyTaxCategory, goblTaxCategoryCode(ctc.ID.Value))

		// The exemption reason (BT-121) is carried at the document level
		// (TaxTotal subtotal); look it up for this line's category.
		key := ubl.BuildTaxCategoryKey(ctc.TaxScheme.ID.Value, ctc.ID.Value, ctc.Percent)
		if info, ok := taxCategoryMap[key]; ok && info.exemptionReasonCode != "" {
			line.Taxes[0].Ext = line.Taxes[0].Ext.Set(cef.ExtKeyVATEX, cbc.Code(info.exemptionReasonCode))
		}
	}
	if ctc.Percent != nil {
		percentStr := ubl.NormalizeNumericString(*ctc.Percent)
		if !strings.HasSuffix(percentStr, "%") {
			percentStr += "%"
		}
		percent, _ := num.PercentageFromString(percentStr)

		// Skip 0% unless zero-rated, so GOBL doesn't normalize exempt/reverse-charge
		// to "zero"; compare via goblTaxCategoryCode to catch the "ZeroRated" wire value.
		if percent.IsZero() && ctc.ID != nil && goblTaxCategoryCode(ctc.ID.Value) != "Z" {
			return
		}

		if line.Taxes == nil {
			line.Taxes = make([]*tax.Combo, 1)
			line.Taxes[0] = &tax.Combo{}
		}
		line.Taxes[0].Percent = &percent
	}
}

// Identical to gobl.ubl.goblItemIdentities.
func goblItemIdentities(di *Item) []*org.Identity {
	ids := make([]*org.Identity, 0)

	if di.BuyersItemIdentification != nil && di.BuyersItemIdentification.ID != nil {
		id := goblIdentity(di.BuyersItemIdentification.ID)
		if id != nil {
			ids = append(ids, id)
		}
	}

	if di.StandardItemIdentification != nil &&
		di.StandardItemIdentification.ID != nil &&
		di.StandardItemIdentification.ID.SchemeID != nil {
		s := *di.StandardItemIdentification.ID.SchemeID
		id := &org.Identity{
			Ext: tax.ExtensionsOf(cbc.CodeMap{
				iso.ExtKeySchemeID: cbc.Code(s),
			}),
			Code: cbc.Code(di.StandardItemIdentification.ID.Value),
		}

		ids = append(ids, id)

	}

	if di.CommodityClassification != nil && len(*di.CommodityClassification) > 0 {
		for _, classification := range *di.CommodityClassification {
			id := goblIdentity(classification.ItemClassificationCode)
			if id != nil {
				ids = append(ids, id)
			}
		}
	}

	return ids
}

// Identical to gobl.ubl.goblIdentity.
func goblIdentity(id *IDType) *org.Identity {
	if id == nil {
		return nil
	}
	identity := &org.Identity{
		Code: cbc.Code(id.Value),
	}
	for _, field := range []*string{id.SchemeID, id.ListID, id.ListVersionID, id.SchemeName, id.Name} {
		if field != nil {
			identity.Label = *field
			break
		}
	}
	return identity
}

// Identical to gobl.ubl.goblLineCharges.
func goblLineCharges(allowances []*AllowanceCharge, line *bill.Line) (*bill.Line, error) {
	for _, ac := range allowances {
		if ac.ChargeIndicator {
			charge, err := goblLineCharge(ac)
			if err != nil {
				return nil, err
			}
			if line.Charges == nil {
				line.Charges = make([]*bill.LineCharge, 0)
			}
			line.Charges = append(line.Charges, charge)
		} else {
			discount, err := goblLineDiscount(ac)
			if err != nil {
				return nil, err
			}
			if line.Discounts == nil {
				line.Discounts = make([]*bill.LineDiscount, 0)
			}
			line.Discounts = append(line.Discounts, discount)
		}
	}
	return line, nil
}
