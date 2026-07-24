package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
)

// Returns excise duties extracted from each line's own cac:TaxTotal, keyed
// by line index, and records ordinary VAT rates into vatPercents.
func (ui *Invoice) stripLineFlavor(vatPercents map[string]string) (map[int][]exciseDuty, error) {
	lines := ui.InvoiceLines
	if len(ui.CreditNoteLines) > 0 {
		lines = ui.CreditNoteLines
	}

	lineExcises := make(map[int][]exciseDuty)
	for i := range lines {
		line := &lines[i]

		vat, excises, err := splitExciseTaxTotals(line.TaxTotal)
		if err != nil {
			return nil, err
		}
		line.TaxTotal = vat
		if len(excises) > 0 {
			lineExcises[i] = excises
		}
		collectVATPercents(line.TaxTotal, vatPercents)
		for j := range line.TaxTotal {
			stripTaxTotalCategories(&line.TaxTotal[j])
		}

		if line.Item != nil {
			if line.Item.ClassifiedTaxCategory != nil {
				stripClassifiedTaxCategoryWire(line.Item.ClassifiedTaxCategory)
			} else {
				synthesizeClassifiedTaxCategory(line)
			}
		}

		stripLineAllowanceCharges(line)
	}
	return lineExcises, nil
}

// Covers a line that states VAT only via its own cac:TaxTotal, not
// cac:ClassifiedTaxCategory (which is all the generic parser reads).
func synthesizeClassifiedTaxCategory(line *ubl.InvoiceLine) {
	for _, tt := range line.TaxTotal {
		for i := range tt.TaxSubtotal {
			tc := &tt.TaxSubtotal[i].TaxCategory
			if tc.ID == nil || tc.TaxScheme == nil {
				continue
			}
			line.Item.ClassifiedTaxCategory = &ubl.ClassifiedTaxCategory{
				ID:        tc.ID,
				Percent:   tc.Percent,
				TaxScheme: tc.TaxScheme,
			}
			return
		}
	}
}

// Folds a line/price-level cac:AllowanceCharge's reason into a note instead
// of a real charge/discount: per G17 3.2/3.3 these are purely advisory.
func stripLineAllowanceCharges(line *ubl.InvoiceLine) {
	allowances := line.AllowanceCharge
	if line.Price != nil && line.Price.AllowanceCharge != nil {
		allowances = append(allowances[:len(allowances):len(allowances)], line.Price.AllowanceCharge)
	}
	for _, ac := range allowances {
		if ac == nil || ac.AllowanceChargeReason == nil || *ac.AllowanceChargeReason == "" {
			continue
		}
		line.Note = append(line.Note, *ac.AllowanceChargeReason)
	}
	line.AllowanceCharge = nil
	if line.Price != nil {
		line.Price.AllowanceCharge = nil
	}
}
