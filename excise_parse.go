package dkoioubl

import (
	"strconv"

	oioubl "github.com/invopop/gobl.dk.oioubl/addon"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

// Legacy UNCL5305 numeric tax-category codes (3010-3671) some real inbound
// documents still use instead of "Excise".
const (
	legacyExciseCategoryMin = 3010
	legacyExciseCategoryMax = 3671
)

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

// Partitions a cac:TaxTotal list into ordinary VAT subtotals (kept) and
// excise duties (extracted, since EN16931 has no field for them).
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

// Records each non-excise subtotal's category and percent, so a document-level
// excise duty's TaxTypeCode (a category only, never a rate) can resolve one.
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

func parseExciseSubtotal(st *ubl.TaxSubtotal) (exciseDuty, error) {
	var d exciseDuty
	amount, err := num.AmountFromString(normalizeNumericString(st.TaxAmount.Value))
	if err != nil {
		return d, err
	}
	d.amount = amount
	if st.TaxableAmount.Value != "" {
		base, err := num.AmountFromString(normalizeNumericString(st.TaxableAmount.Value))
		if err != nil {
			return d, err
		}
		d.base = &base
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

// Keys a duty by scheme+amount+base, to tell a document-level mirror of a
// line-level duty (outbound emits both) from a genuine document-level one.
func exciseMirrorKey(d exciseDuty) string {
	base := ""
	if d.base != nil {
		base = d.base.String()
	}
	return d.scheme + "|" + d.amount.String() + "|" + base
}

// No Base here: bill.LineCharge.Base only means something alongside Percent,
// and an excise duty is always a fixed Amount.
func dutyToLineCharge(d exciseDuty) *bill.LineCharge {
	return &bill.LineCharge{
		Key:    oioubl.ChargeKeyExcise,
		Ext:    dutyCodeExt(d.scheme),
		Reason: d.name,
		Amount: d.amount,
	}
}

// A document-level duty states its own VAT category via TaxTypeCode (a
// line-level duty inherits its line's category instead, no recovery needed).
func dutyToCharge(d exciseDuty, percents map[string]string) *bill.Charge {
	ch := &bill.Charge{
		Key:    oioubl.ChargeKeyExcise,
		Ext:    dutyCodeExt(d.scheme),
		Reason: d.name,
		Amount: d.amount,
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
	return tax.ExtensionsOf(cbc.CodeMap{oioubl.ExtKeyDutyCode: cbc.Code(scheme)})
}

// Adds each line's duties to that line, then the genuine document-level ones
// (skipping any that just mirror a duty already added to a line).
func addExciseCharges(inv *bill.Invoice, docExcises []exciseDuty, lineExcises map[int][]exciseDuty, vatPercents map[string]string) {
	mirrors := make(map[string]bool)
	for i, duties := range lineExcises {
		if i >= len(inv.Lines) {
			continue
		}
		for _, d := range duties {
			inv.Lines[i].Charges = append(inv.Lines[i].Charges, dutyToLineCharge(d))
			mirrors[exciseMirrorKey(d)] = true
		}
	}
	for _, d := range docExcises {
		if mirrors[exciseMirrorKey(d)] {
			continue
		}
		inv.Charges = append(inv.Charges, dutyToCharge(d, vatPercents))
	}
}
