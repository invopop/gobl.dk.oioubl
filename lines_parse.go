package dkoioubl

import (
	"strings"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/cef"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
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

	taxCategoryMap := (*ubl.Invoice)(ui).BuildTaxCategoryMap()

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

// goblConvertLine also reconstructs line-level cac:TaxTotal/Excise blocks as
// line charges (see exciseLineChargesFromTaxTotals).
func goblConvertLine(docLine *InvoiceLine, taxCategoryMap map[string]string) (*bill.Line, error) {
	if docLine.Price == nil {
		return nil, nil
	}
	price, err := goblLinePrice(docLine.Price)
	if err != nil {
		return nil, err
	}

	line := &bill.Line{
		Quantity: num.MakeAmount(1, 0),
		Item:     &org.Item{Price: &price},
	}
	if di := docLine.Item; di != nil {
		ubl.GoblConvertLineItem(di, line.Item)
		goblApplyLineTaxCategory(di.ClassifiedTaxCategory, line, taxCategoryMap)
	}
	// cac:ClassifiedTaxCategory is authoritative when present, but most real
	// OIOUBL documents state line VAT only in the line's own cac:TaxTotal.
	if len(line.Taxes) == 0 {
		goblLineTaxesFromTaxTotals(docLine.TaxTotal, line, taxCategoryMap)
	}

	if err := goblLineQuantity(line, docLine); err != nil {
		return nil, err
	}
	line.Notes = goblLineNotes(docLine.Note)

	if docLine.AccountingCost != nil {
		line.Cost = cbc.Code(*docLine.AccountingCost) // BT-133
	}
	goblLineDocumentReference(line, docLine)

	if docLine.InvoicePeriod != nil {
		line.Period, err = ubl.GoblPeriodDates(docLine.InvoicePeriod)
		if err != nil {
			return nil, err
		}
	}
	if docLine.OrderLineReference != nil && docLine.OrderLineReference.LineID != "" {
		line.Order = cbc.Code(docLine.OrderLineReference.LineID)
	}

	// cac:AllowanceCharge can sit directly under the line, and/or nested under
	// cac:Price (a price-level adjustment baked into the unit price, e.g. a
	// packaging fee); both map onto the line the same way.
	allowances := docLine.AllowanceCharge
	if docLine.Price.AllowanceCharge != nil {
		allowances = append(allowances[:len(allowances):len(allowances)], docLine.Price.AllowanceCharge)
	}
	if len(allowances) > 0 {
		line, err = goblLineCharges(allowances, line)
		if err != nil {
			return nil, err
		}
	}

	excise, err := exciseLineChargesFromTaxTotals(docLine.TaxTotal)
	if err != nil {
		return nil, err
	}
	line.Charges = append(line.Charges, excise...)

	return line, nil
}

// goblLinePrice reads the line's unit price, rescaling for a BaseQuantity
// other than 1 to avoid rounding loss (see calculateRequiredPrecision).
func goblLinePrice(p *Price) (num.Amount, error) {
	price, err := num.AmountFromString(ubl.NormalizeNumericString(p.PriceAmount.Value))
	if err != nil {
		return num.Amount{}, err
	}
	if p.BaseQuantity == nil {
		return price, nil
	}
	baseQuantity, err := num.AmountFromString(ubl.NormalizeNumericString(p.BaseQuantity.Value))
	if err != nil {
		return num.Amount{}, err
	}
	if baseQuantity.IsZero() {
		return price, nil
	}
	precision := ubl.CalculateRequiredPrecision(price, baseQuantity)
	return price.RescaleUp(precision).Divide(baseQuantity), nil
}

// goblLineQuantity reads the (credited or invoiced) quantity and its unit.
func goblLineQuantity(line *bill.Line, docLine *InvoiceLine) error {
	iq := docLine.InvoicedQuantity
	if docLine.CreditedQuantity != nil {
		iq = docLine.CreditedQuantity
	}
	if iq == nil {
		return nil
	}
	q, err := num.AmountFromString(ubl.NormalizeNumericString(iq.Value))
	if err != nil {
		return err
	}
	line.Quantity = q
	if iq.UnitCode != "" {
		line.Item.Unit = ubl.GoblUnitFromUNECE(cbc.Code(iq.UnitCode))
	}
	return nil
}

func goblLineNotes(docNotes []string) []*org.Note {
	var notes []*org.Note
	for _, note := range docNotes {
		if note != "" {
			notes = append(notes, &org.Note{Text: ubl.CleanString(note)})
		}
	}
	return notes
}

// goblLineDocumentReference maps a line's own object identifier (BT-128).
func goblLineDocumentReference(line *bill.Line, docLine *InvoiceLine) {
	if docLine.DocumentReference == nil || docLine.DocumentReference.ID.Value == "" {
		return
	}
	line.Identifier = &org.Identity{Code: cbc.Code(docLine.DocumentReference.ID.Value)}
	if docLine.DocumentReference.ID.SchemeID != nil {
		line.Identifier.Ext = tax.ExtensionsOf(cbc.CodeMap{
			untdid.ExtKeyReference: cbc.Code(*docLine.DocumentReference.ID.SchemeID),
		})
	}
}

// goblLineTaxesFromTaxTotals falls back to the line's own cac:TaxTotal for VAT
// when it carries no cac:ClassifiedTaxCategory (excise subtotals are skipped).
func goblLineTaxesFromTaxTotals(totals []TaxTotal, line *bill.Line, taxCategoryMap map[string]string) {
	for _, tt := range totals {
		for i := range tt.TaxSubtotal {
			tc := &tt.TaxSubtotal[i].TaxCategory
			if tc.ID == nil || tc.TaxScheme == nil || isExciseCategoryID(tc.ID.Value) {
				continue
			}
			goblApplyLineTaxCategory(&ClassifiedTaxCategory{
				ID:        tc.ID,
				Percent:   tc.Percent,
				TaxScheme: tc.TaxScheme,
			}, line, taxCategoryMap)
			return
		}
	}
}

// goblApplyLineTaxCategory maps a tax category onto the line's taxes via
// goblTaxSchemeCategory/goblTaxCategoryCode.
func goblApplyLineTaxCategory(ctc *ClassifiedTaxCategory, line *bill.Line, taxCategoryMap map[string]string) {
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
		if code, ok := taxCategoryMap[key]; ok && code != "" {
			line.Taxes[0].Ext = line.Taxes[0].Ext.Set(cef.ExtKeyVATEX, cbc.Code(code))
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
