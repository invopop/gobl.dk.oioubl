package oioubl

import (
	ubl "github.com/invopop/gobl.ubl"
)

// stripLines pulls each line's excise duties out, keyed by line number, and
// notes the ordinary VAT rates it saw along the way.
func (ui *Invoice) stripLines(vatPercents map[string]string) (map[int][]exciseDuty, error) {
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

// synthesizeClassifiedTaxCategory covers a line that states its VAT only in a
// tax total, which is not where the generic parser looks.
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

// stripLineAllowanceCharges turns a line's allowances into a note rather than
// real money, because OIOUBL means them as information only (G17 3.2/3.3).
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
