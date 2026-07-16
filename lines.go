package dkoioubl

import (
	"strconv"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/currency"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

// OIOUBL: gross line total (F-INV348) and no line-level allowances (promoted to document level, F-INV126/128/129).
func (ui *Invoice) addLines(inv *bill.Invoice) {
	if len(inv.Lines) == 0 {
		return
	}

	var lines []InvoiceLine
	invoiceType := ui.getInvoiceTypeBasedOnXMLName()

	for _, l := range inv.Lines {
		ccy := l.Item.Currency.String()
		if ccy == "" {
			ccy = inv.Currency.String()
		}
		// F-INV348: gross Price×Qty here; line allowances net at the document level.
		lineExt := l.Total.String()
		if l.Sum != nil {
			lineExt = l.Sum.String()
		}
		invLine := InvoiceLine{
			ID: strconv.Itoa(l.Index),

			LineExtensionAmount: Amount{
				CurrencyID: &ccy,
				Value:      lineExt,
			},
		}

		iq := &Quantity{
			Value: l.Quantity.String(),
		}
		if l.Item != nil && l.Item.Unit != "" {
			iq.UnitCode = string(l.Item.Unit.UNECE())
		}
		if invoiceType.In(bill.InvoiceTypeCreditNote) {
			invLine.CreditedQuantity = iq
		} else {
			invLine.InvoicedQuantity = iq
		}

		if len(l.Notes) > 0 {
			var notes []string
			for _, note := range l.Notes {
				if note.Key == "buyer-accounting-ref" {
					invLine.AccountingCost = &note.Text
				} else {
					notes = append(notes, note.Text)
				}
			}
			if len(notes) > 0 {
				invLine.Note = notes
			}
		}

		// BT-128: Invoice line object identifier
		if l.Identifier != nil {
			typeCode := "130"
			ref := &LineDocReference{
				ID:               IDType{Value: l.Identifier.Code.String()},
				DocumentTypeCode: &typeCode,
			}
			if l.Identifier.Ext.Has(untdid.ExtKeyReference) {
				s := l.Identifier.Ext.Get(untdid.ExtKeyReference).String()
				ref.ID.SchemeID = &s
			}
			invLine.DocumentReference = ref
		}

		if l.Period != nil {
			invLine.InvoicePeriod = &Period{
				StartDate: ubl.FormatDate(l.Period.Start),
				EndDate:   ubl.FormatDate(l.Period.End),
			}
		}

		if l.Order != "" {
			invLine.OrderLineReference = &OrderLineReference{
				LineID: l.Order.String(),
			}
		}

		// OIOUBL reconciles allowances/charges at the document level (F-INV126/128/129),
		// so they're promoted in addTotals instead of set on the line.

		if l.Item != nil {
			it := &Item{}

			if l.Item.Description != "" {
				d := l.Item.Description
				it.Description = &d
			}

			if l.Item.Name != "" {
				it.Name = l.Item.Name
			}

			// OIOUBL forbids cac:OriginCountry on a line item (F-INV211 / F-CRN109).

			if l.Item.Meta != nil {
				var properties []AdditionalItemProperty
				for key, value := range l.Item.Meta {
					properties = append(properties, AdditionalItemProperty{Name: key.String(), Value: value})
				}
				it.AdditionalItemProperty = &properties
			}

			if len(l.Taxes) > 0 && l.Taxes[0].Category != "" {
				it.ClassifiedTaxCategory = &ClassifiedTaxCategory{
					TaxScheme: &TaxScheme{
						ID: IDType{Value: l.Taxes[0].Category.String()},
					},
				}

				if cat := taxCategoryID(l.Taxes[0].Key); cat != "" {
					it.ClassifiedTaxCategory.ID = &IDType{Value: cat}
				}

				// Set percent: required unless category is "O" (outside scope)
				if l.Taxes[0].Percent != nil {
					p := l.Taxes[0].Percent.StringWithoutSymbol()
					it.ClassifiedTaxCategory.Percent = &p
				} else if it.ClassifiedTaxCategory.ID == nil || it.ClassifiedTaxCategory.ID.Value != "O" {
					p := "0"
					it.ClassifiedTaxCategory.Percent = &p
				}
			}

			if len(l.Item.Identities) > 0 {
				for _, id := range l.Item.Identities {
					// BT-158/159: Item classification (Label holds the listID)
					if id.Label != "" && !id.Ext.Has(iso.ExtKeySchemeID) {
						listID := id.Label
						if it.CommodityClassification == nil {
							it.CommodityClassification = &[]CommodityClassification{}
						}
						*it.CommodityClassification = append(*it.CommodityClassification, CommodityClassification{
							ItemClassificationCode: &IDType{
								Value:  id.Code.String(),
								ListID: &listID,
							},
						})
						continue
					}

					if it.BuyersItemIdentification != nil && it.StandardItemIdentification != nil {
						break
					}

					s := id.Ext.Get(iso.ExtKeySchemeID).String()

					// First identity without extension → BuyersItemIdentification.
					if s == "" {
						if it.BuyersItemIdentification == nil {
							it.BuyersItemIdentification = &ItemIdentification{
								ID: &IDType{
									Value: id.Code.String(),
								},
							}
						}
						continue
					}

					// First identity with extension → StandardItemIdentification.
					if it.StandardItemIdentification == nil {
						it.StandardItemIdentification = &ItemIdentification{
							ID: &IDType{
								SchemeID: &s,
								Value:    id.Code.String(),
							},
						}
					}
				}
			}

			invLine.Item = it

			if l.Item.Price != nil {
				invLine.Price = &Price{
					PriceAmount: Amount{
						CurrencyID: &ccy,
						Value:      l.Item.Price.String(),
					},
				}
			}

			if l.Item.Ref != "" {
				invLine.Item.SellersItemIdentification = &ItemIdentification{
					ID: &IDType{
						Value: l.Item.Ref.String(),
					},
				}
			}
		}

		invLine.TaxTotal = makeLineTaxTotals(l, ccy)

		lines = append(lines, invLine)
	}
	if invoiceType.In(bill.InvoiceTypeCreditNote) {
		ui.CreditNoteLines = lines
	} else {
		ui.InvoiceLines = lines
	}

	applyLineTaxCategories(ui.InvoiceLines)
	applyLineTaxCategories(ui.CreditNoteLines)
}

// applyLineTaxCategories stamps the tax categories on each line's classified
// category, line-level subtotals, and promoted allowance/charges.
func applyLineTaxCategories(lines []InvoiceLine) {
	for i := range lines {
		line := &lines[i]
		if line.Item != nil && line.Item.ClassifiedTaxCategory != nil {
			applyClassifiedTaxCategory(line.Item.ClassifiedTaxCategory)
		}
		for j := range line.TaxTotal {
			for k := range line.TaxTotal[j].TaxSubtotal {
				st := &line.TaxTotal[j].TaxSubtotal[k]
				// Excise subtotals carry their own scheme code, name and TaxTypeCode;
				// the VAT overlay would clobber them with 63/Moms, so leave them be.
				if st.TaxCategory.ID != nil && st.TaxCategory.ID.Value == taxCategoryExcise {
					continue
				}
				applyTaxCategory(&st.TaxCategory)
			}
		}
		for _, ac := range line.AllowanceCharge {
			for _, tc := range ac.TaxCategory {
				applyTaxCategory(tc)
			}
		}
	}
}

// rescaleToCurrency rounds the amount to the currency's natural precision
// (2 for EUR, 0 for JPY), or leaves it unchanged for an unknown currency.
func rescaleToCurrency(a num.Amount, ccy string) num.Amount {
	if def := currency.Code(ccy).Def(); def != nil {
		return def.Rescale(a)
	}
	return a
}

// makeLineTaxTotals builds the OIOUBL line-level cac:TaxTotal, required on
// every line, even at 0% (F-INV138 / F-LIB404).
func makeLineTaxTotals(line *bill.Line, ccy string) []TaxTotal {
	if line == nil || len(line.Taxes) == 0 {
		return nil
	}

	var taxable num.Amount
	switch {
	case line.Sum != nil:
		// Line TaxableAmount is gross (Price×Qty); the discount is taken once at document level (F-LIB402).
		taxable = *line.Sum
	case line.Total != nil:
		taxable = *line.Total
	default:
		return nil
	}

	// An excise duty is emitted as its own tax, not an AllowanceCharge, so fold
	// it into the VAT taxable base here: VAT lands on the duty-inclusive amount (F-LIB402).
	for _, ch := range line.Charges {
		if chargeExciseScheme(ch.Key) != "" {
			taxable = taxable.Add(rescaleToCurrency(ch.Amount, ccy))
		}
	}

	taxTotal := TaxTotal{
		TaxAmount: Amount{Value: "0", CurrencyID: &ccy},
	}
	totalAmount := num.MakeAmount(0, taxable.Exp())

	for _, t := range line.Taxes {
		subtotal := TaxSubtotal{
			TaxableAmount: Amount{Value: taxable.String(), CurrencyID: &ccy},
		}
		taxCat := TaxCategory{}

		if k := taxCategoryID(t.Key); k != "" {
			taxCat.ID = &IDType{Value: k}
		}

		if t.Percent != nil {
			p := t.Percent.StringWithoutSymbol()
			taxCat.Percent = &p
			amount := t.Percent.Of(taxable).Rescale(taxable.Exp())
			subtotal.TaxAmount = Amount{Value: amount.String(), CurrencyID: &ccy}
			totalAmount = totalAmount.Add(amount)
		} else {
			// No percent (e.g. exempt): still emit at currency precision
			// ("0.00"), or OIOUBL F-LIB263 rejects a bare "0".
			subtotal.TaxAmount = Amount{Value: num.MakeAmount(0, taxable.Exp()).String(), CurrencyID: &ccy}
		}

		if t.Category != "" {
			taxCat.TaxScheme = &TaxScheme{ID: IDType{Value: t.Category.String()}}
		}
		subtotal.TaxCategory = taxCat
		taxTotal.TaxSubtotal = append(taxTotal.TaxSubtotal, subtotal)
	}

	taxTotal.TaxAmount = Amount{Value: totalAmount.String(), CurrencyID: &ccy}

	// Also emit line-level cac:TaxTotal/Excise blocks so the wire records which line each duty belongs to.
	totals := []TaxTotal{taxTotal}
	totals = append(totals, makeExciseTaxTotals(collectLineExcise(line, ccy), ccy)...)
	return totals
}

// OIOUBL: stamps a TaxCategory on each line allowance/charge (F-LIB226).
func makeLineCharges(charges []*bill.LineCharge, discounts []*bill.LineDiscount, ccy string, baseSum *num.Amount, taxes tax.Set) []*AllowanceCharge {
	var allowanceCharges []*AllowanceCharge
	// BR-DEC-24 / UBL-DT-01: GOBL only clamps line charge/discount amounts to
	// the item price's precision, which can exceed the currency's, so they are
	// rescaled here; the base (the line sum) is already at currency precision.
	var base *Amount
	if baseSum != nil {
		base = &Amount{
			Value:      baseSum.String(),
			CurrencyID: &ccy,
		}
	}
	for _, ch := range charges {
		ac := &AllowanceCharge{
			ChargeIndicator: true,
			Amount: Amount{
				Value:      rescaleToCurrency(ch.Amount, ccy).String(),
				CurrencyID: &ccy,
			},
		}
		if e := ch.Ext.Get(untdid.ExtKeyCharge).String(); e != "" {
			ac.AllowanceChargeReasonCode = &e
		}
		if ch.Reason != "" {
			ac.AllowanceChargeReason = &ch.Reason
		}
		if ch.Percent != nil {
			p := allowanceMultiplier(ch.Percent)
			ac.MultiplierFactorNumeric = &p
			if base != nil {
				ac.BaseAmount = base
			}
		}
		ac.TaxCategory = makeTaxCategory(taxes) // F-LIB226: line allowance needs a TaxCategory
		allowanceCharges = append(allowanceCharges, ac)
	}
	for _, d := range discounts {
		ac := &AllowanceCharge{
			ChargeIndicator: false,
			Amount: Amount{
				Value:      rescaleToCurrency(d.Amount, ccy).String(),
				CurrencyID: &ccy,
			},
		}
		if e := d.Ext.Get(untdid.ExtKeyAllowance).String(); e != "" {
			ac.AllowanceChargeReasonCode = &e
		}
		if d.Reason != "" {
			ac.AllowanceChargeReason = &d.Reason
		}
		if d.Percent != nil {
			p := allowanceMultiplier(d.Percent)
			ac.MultiplierFactorNumeric = &p
			if base != nil {
				ac.BaseAmount = base
			}
		}
		ac.TaxCategory = makeTaxCategory(taxes) // F-LIB226: line allowance needs a TaxCategory
		allowanceCharges = append(allowanceCharges, ac)
	}
	return allowanceCharges
}
