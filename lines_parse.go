package oioubl

import (
	"fmt"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/num"
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

		// The generic parser silently drops a line with no price, losing its
		// amount and shifting every later line's duties, which are keyed by
		// index here. Refusing is the honest failure.
		if line.Price == nil {
			return nil, fmt.Errorf("line %d has no price", i+1)
		}

		if err := foldOrderableUnitFactor(line.Price); err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}

		vat, excises, err := splitExciseTaxTotals(line.TaxTotal)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
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
				stripClassifiedTaxCategory(line.Item.ClassifiedTaxCategory)
			} else {
				synthesizeClassifiedTaxCategory(line)
			}
		}

		stripLineAllowanceCharges(line)
	}
	return lineExcises, nil
}

// foldOrderableUnitFactor rewrites an OIOUBL price as the plain per-unit price
// the generic parser expects. OIOUBL prices the invoiced unit as PriceAmount *
// OrderableUnitFactorRate -- the factor already absorbs the base quantity, per
// G25's own reduction of PriceAmount / BaseQuantity * (BaseQuantity * factor)
// -- so the generic parser's division by BaseQuantity would count it twice. A
// crate invoiced at 1 CS, priced 60.00 per bottle with a factor of 12, is a
// 720.00 line; without the fold it converts as a 60.00 one.
func foldOrderableUnitFactor(price *ubl.Price) error {
	price.BaseQuantity = nil
	if price.OrderableUnitFactorRate == nil {
		return nil
	}
	factor, err := num.AmountFromString(normalizeNumericString(*price.OrderableUnitFactorRate))
	if err != nil {
		return fmt.Errorf("orderable unit factor %q: %w", *price.OrderableUnitFactorRate, err)
	}
	price.OrderableUnitFactorRate = nil
	if factor.IsZero() || factor.Compare(num.MakeAmount(1, 0)) == 0 {
		return nil
	}
	amount, err := num.AmountFromString(normalizeNumericString(price.PriceAmount.Value))
	if err != nil {
		return fmt.Errorf("price amount %q: %w", price.PriceAmount.Value, err)
	}
	price.PriceAmount.Value = amount.Multiply(factor).String()
	return nil
}

// synthesizeClassifiedTaxCategory covers a line that states its VAT only in a
// tax total, which is not where the generic parser looks.
func synthesizeClassifiedTaxCategory(line *ubl.InvoiceLine) {
	var first, wholeLine *ubl.TaxCategory
	for _, tt := range line.TaxTotal {
		for i := range tt.TaxSubtotal {
			st := &tt.TaxSubtotal[i]
			if st.TaxCategory.ID == nil || st.TaxCategory.TaxScheme == nil {
				continue
			}
			if first == nil {
				first = &st.TaxCategory
			}
			// A line may state more than one category, the others covering its
			// duties; the line's own is the one levied on the whole line.
			if wholeLine == nil && sameWireAmount(st.TaxableAmount.Value, line.LineExtensionAmount.Value) {
				wholeLine = &st.TaxCategory
			}
		}
	}
	tc := wholeLine
	if tc == nil {
		tc = first
	}
	if tc == nil {
		return
	}
	line.Item.ClassifiedTaxCategory = &ubl.ClassifiedTaxCategory{
		ID:        tc.ID,
		Percent:   tc.Percent,
		TaxScheme: tc.TaxScheme,
	}
}

// sameWireAmount reports whether two wire amounts are numerically equal, so
// "200000" and "200000.00" match.
func sameWireAmount(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	x, err := num.AmountFromString(normalizeNumericString(a))
	if err != nil {
		return false
	}
	y, err := num.AmountFromString(normalizeNumericString(b))
	if err != nil {
		return false
	}
	return x.Compare(y) == 0
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
