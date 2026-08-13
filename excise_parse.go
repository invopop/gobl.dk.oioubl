package oioubl

import (
	"fmt"
	"strconv"

	"github.com/invopop/gobl.dk.oioubl/addon"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

// Older documents mark excise with a numeric code instead of "Excise".
const (
	legacyExciseCategoryMin = 3010
	legacyExciseCategoryMax = 3671
)

// splitExciseTaxTotals separates ordinary VAT, which is kept, from excise
// duties, which are pulled out because EN 16931 has nowhere to put them.
func splitExciseTaxTotals(totals []ubl.TaxTotal) ([]ubl.TaxTotal, []exciseDuty, error) {
	var kept []ubl.TaxTotal
	var excises []exciseDuty
	for _, tt := range totals {
		var vat []ubl.TaxSubtotal
		for i := range tt.TaxSubtotal {
			st := &tt.TaxSubtotal[i]
			if st.TaxCategory.ID == nil || !isExciseCategoryID(st.TaxCategory.ID.Value) {
				vat = append(vat, *st)
				continue
			}
			d, err := parseExciseSubtotal(st)
			if err != nil {
				return nil, nil, err
			}
			excises = append(excises, d)
		}
		if len(vat) == 0 {
			continue
		}
		tt.TaxSubtotal = vat
		kept = append(kept, tt)
	}
	return kept, excises, nil
}

func parseExciseSubtotal(st *ubl.TaxSubtotal) (exciseDuty, error) {
	var d exciseDuty
	amount, err := num.AmountFromString(normalizeNumericString(st.TaxAmount.Value))
	if err != nil {
		return d, fmt.Errorf("excise duty tax amount %q: %w", st.TaxAmount.Value, err)
	}
	d.amount = amount
	if st.TaxableAmount.Value != "" {
		base, err := num.AmountFromString(normalizeNumericString(st.TaxableAmount.Value))
		if err != nil {
			return d, fmt.Errorf("excise duty taxable amount %q: %w", st.TaxableAmount.Value, err)
		}
		d.base = &base
	}
	if p := st.TaxCategory.Percent; p != nil {
		if percent, err := num.PercentageFromString(normalizeNumericString(*p) + "%"); err == nil {
			d.percent = &percent
		}
	}
	if ts := st.TaxCategory.TaxScheme; ts != nil {
		d.scheme = ts.ID.Value
		if ts.Name != nil {
			d.name = *ts.Name
		}
		if ts.TaxTypeCode != nil {
			d.typeCode = ts.TaxTypeCode.Value
		}
	}
	return d, nil
}

// isExciseCategoryID reports whether a taxcategoryid-1.1 value marks a
// non-VAT excise duty rather than an ordinary VAT rate.
func isExciseCategoryID(id string) bool {
	if id == taxCategoryExcise {
		return true
	}
	n, err := strconv.Atoi(id)
	if err != nil {
		return false
	}
	return n >= legacyExciseCategoryMin && n <= legacyExciseCategoryMax
}

// collectVATPercents notes each VAT rate, so a document-level duty that names
// only a category can later find the matching percentage.
func collectVATPercents(totals []ubl.TaxTotal, percents map[string]string) {
	for _, tt := range totals {
		for i := range tt.TaxSubtotal {
			tc := &tt.TaxSubtotal[i].TaxCategory
			if tc.ID == nil || tc.Percent == nil || isExciseCategoryID(tc.ID.Value) {
				continue
			}
			if _, ok := percents[tc.ID.Value]; !ok {
				percents[tc.ID.Value] = *tc.Percent
			}
		}
	}
}

// addExciseCharges adds every duty exactly once. A document-level entry that
// mirrors a line's duty and states the VAT type the line already implies is a
// pure restatement: the line keeps its duty, and with it the record of which
// line the duty belongs to. Only when the stated VAT type differs from the
// line's own rate does the document level win, because a line charge has no
// taxes of its own and would inherit the line's rate.
func addExciseCharges(inv *bill.Invoice, details oioublDetails) {
	consumed := make([]bool, len(details.docDuties))
	for i := range inv.Lines {
		for _, d := range details.lineDuties[i] {
			j := matchingDocDuty(details.docDuties, consumed, d)
			if j >= 0 && unclTaxCategoryCode(details.docDuties[j].typeCode) == details.lineVATCodes[i] {
				consumed[j] = true
				inv.Lines[i].Charges = append(inv.Lines[i].Charges, dutyToLineCharge(d))
				continue
			}
			if j >= 0 {
				continue
			}
			inv.Lines[i].Charges = append(inv.Lines[i].Charges, dutyToLineCharge(d))
		}
	}
	for j, d := range details.docDuties {
		if consumed[j] {
			continue
		}
		inv.Charges = append(inv.Charges, dutyToCharge(d, details.vatPercents))
	}
}

// matchingDocDuty finds a document-level entry restating a line duty. Amounts
// compare numerically, so a mirror written "50000.0" still matches its line's
// "50000.00".
func matchingDocDuty(docDuties []exciseDuty, consumed []bool, d exciseDuty) int {
	for j, doc := range docDuties {
		if !consumed[j] && doc.scheme == d.scheme && doc.amount.Compare(d.amount) == 0 && sameDutyBase(doc.base, d.base) {
			return j
		}
	}
	return -1
}

func sameDutyBase(a, b *num.Amount) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Compare(*b) == 0
}

func dutyToLineCharge(d exciseDuty) *bill.LineCharge {
	lc := &bill.LineCharge{
		Key:    addon.ChargeKeyExcise,
		Ext:    dutyCodeExt(d.scheme),
		Reason: d.name,
		Amount: d.amount,
	}
	// Same rule as dutyToCharge: the taxable base can only be kept for a flat
	// rate, since GOBL demands a percent with it and recomputes the amount.
	if d.base != nil && d.percent != nil && d.percent.Of(*d.base).Compare(d.amount) == 0 {
		lc.Base = d.base
		lc.Percent = d.percent
	}
	return lc
}

// dutyToCharge builds a document-level duty, which unlike a line-level one has
// to state its own VAT category.
func dutyToCharge(d exciseDuty, percents map[string]string) *bill.Charge {
	ch := &bill.Charge{
		Key:    addon.ChargeKeyExcise,
		Ext:    dutyCodeExt(d.scheme),
		Reason: d.name,
		Amount: d.amount,
	}
	// The taxable base can only be kept when the duty is a flat rate: GOBL
	// requires a percent alongside a base and recomputes the amount from them,
	// so a progressive duty like registreringsafgift, whose base and amount
	// relate by no percentage, keeps its amount alone.
	if d.base != nil && d.percent != nil && d.percent.Of(*d.base).Compare(d.amount) == 0 {
		ch.Base = d.base
		ch.Percent = d.percent
	}
	key := oioublVATKey(d.typeCode)
	if key == "" {
		return ch
	}
	combo := &tax.Combo{Category: tax.CategoryVAT, Key: key}
	if p, ok := percents[d.typeCode]; ok {
		if percent, err := num.PercentageFromString(normalizeNumericString(p) + "%"); err == nil {
			combo.Percent = &percent
		}
	}
	ch.Taxes = tax.Set{combo}
	return ch
}

func dutyCodeExt(scheme string) tax.Extensions {
	if scheme == "" {
		return tax.Extensions{}
	}
	return tax.ExtensionsOf(cbc.CodeMap{addon.ExtKeyDutyCode: cbc.Code(scheme)})
}

