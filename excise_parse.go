package oioubl

import (
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

// addExciseCharges puts each duty back on its line, then adds the document-level
// ones, skipping any that merely repeat a line's duty.
func addExciseCharges(inv *bill.Invoice, details oioublDetails) {
	mirrors := make(map[string]bool)
	for i, duties := range details.lineDuties {
		if i >= len(inv.Lines) {
			continue
		}
		for _, d := range duties {
			inv.Lines[i].Charges = append(inv.Lines[i].Charges, dutyToLineCharge(d))
			mirrors[exciseMirrorKey(d)] = true
		}
	}
	for _, d := range details.docDuties {
		if mirrors[exciseMirrorKey(d)] {
			continue
		}
		inv.Charges = append(inv.Charges, dutyToCharge(d, details.vatPercents))
	}
}

// No Base here: bill.LineCharge.Base only means something alongside Percent,
// and an excise duty is always a fixed Amount.
func dutyToLineCharge(d exciseDuty) *bill.LineCharge {
	return &bill.LineCharge{
		Key:    addon.ChargeKeyExcise,
		Ext:    dutyCodeExt(d.scheme),
		Reason: d.name,
		Amount: d.amount,
	}
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

// exciseMirrorKey identifies a duty, so a document-level copy of one already
// counted on a line can be told from a genuine document-level duty.
func exciseMirrorKey(d exciseDuty) string {
	base := ""
	if d.base != nil {
		base = d.base.String()
	}
	return d.scheme + "|" + d.amount.String() + "|" + base
}
