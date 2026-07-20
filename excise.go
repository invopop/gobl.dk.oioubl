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

// taxCategoryExcise is the taxcategoryid-1.1 category OIOUBL emits for a non-VAT
// excise duty (as a cac:TaxTotal, not a cac:AllowanceCharge).
const taxCategoryExcise = "Excise"

// legacyExciseCategoryMin/Max bound the legacy UNCL5305 numeric tax-category
// codes (3010-3671) that predate taxcategoryid-1.1's "Excise" value.
const (
	legacyExciseCategoryMin = 3010
	legacyExciseCategoryMax = 3671
)

// isExciseCategoryID reports whether a taxcategoryid-1.1 value is "Excise" or
// a legacy UNCL5305 duty code; only "Excise" is ever emitted outbound.
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

// exciseDuty is a duty resolved into the values OIOUBL needs.
type exciseDuty struct {
	scheme string
	name   string
	amount num.Amount
	// base is the amount the duty rate was applied to (cac:TaxSubtotal/TaxableAmount).
	base *num.Amount
	// typeCode is the duty's taxtypecode value: a document-level duty states its
	// own, a line-level duty inherits the line's VAT category.
	typeCode string
}

// chargeIsExcise reports whether a charge Key marks an OIOUBL excise duty.
func chargeIsExcise(key cbc.Key) bool {
	return key == oioubl.ChargeKeyExcise
}

// chargeDutyCode returns an excise charge's SKAT duty code (its OIOUBL
// taxschemeid value, e.g. "16"), carried in the duty-code extension.
func chargeDutyCode(ext tax.Extensions) string {
	return ext.Get(oioubl.ExtKeyDutyCode).String()
}

// dutyCodeExt builds the extension carrying a parsed duty code.
func dutyCodeExt(code string) tax.Extensions {
	return tax.ExtensionsOf(cbc.CodeMap{oioubl.ExtKeyDutyCode: cbc.Code(code)})
}

// collectExcise gathers every excise duty across document- and line-level charges.
func collectExcise(inv *bill.Invoice, currency string) []exciseDuty {
	var out []exciseDuty
	for _, ch := range inv.Charges {
		if chargeIsExcise(ch.Key) {
			out = append(out, exciseDuty{
				scheme:   chargeDutyCode(ch.Ext),
				name:     ch.Reason,
				amount:   ch.Amount,
				base:     ch.Base,
				typeCode: chargeVATTypeCode(ch),
			})
		}
	}
	for _, l := range inv.Lines {
		out = append(out, collectLineExcise(l, currency)...)
	}
	return out
}

// collectLineExcise gathers a line's excise duties, mirrored as line-level
// cac:TaxTotal blocks so the wire records which line each duty belongs to.
func collectLineExcise(line *bill.Line, currency string) []exciseDuty {
	var out []exciseDuty
	typeCode := lineVATTypeCode(line)
	for _, ch := range line.Charges {
		if chargeIsExcise(ch.Key) {
			var base *num.Amount
			if ch.Base != nil {
				b := rescaleToCurrency(*ch.Base, currency)
				base = &b
			}
			out = append(out, exciseDuty{
				scheme:   chargeDutyCode(ch.Ext),
				name:     ch.Reason,
				amount:   rescaleToCurrency(ch.Amount, currency),
				base:     base,
				typeCode: typeCode,
			})
		}
	}
	return out
}

// chargeVATTypeCode resolves a document-level duty's own VAT combo into its
// OIOUBL taxtypecode value.
func chargeVATTypeCode(ch *bill.Charge) string {
	if ch == nil {
		return ""
	}
	combo := ch.Taxes.Get(tax.CategoryVAT)
	if combo == nil {
		return ""
	}
	return taxCategoryID(combo.Key)
}

// lineVATTypeCode resolves the taxtypecode a line's duties inherit from the
// line's own VAT category (OIOUBL Skat guideline).
func lineVATTypeCode(line *bill.Line) string {
	if line == nil {
		return ""
	}
	combo := line.Taxes.Get(tax.CategoryVAT)
	if combo == nil {
		return ""
	}
	return taxCategoryID(combo.Key)
}

// exciseVATBases sums, per VAT key, the base document-level excise charges
// contribute to the invoice's tax totals, so addTotals can spot excise-only rows.
func exciseVATBases(inv *bill.Invoice) map[cbc.Key]num.Amount {
	bases := make(map[cbc.Key]num.Amount)
	for _, ch := range inv.Charges {
		if !chargeIsExcise(ch.Key) {
			continue
		}
		combo := ch.Taxes.Get(tax.CategoryVAT)
		if combo == nil {
			continue
		}
		amt := ch.Amount
		if b, ok := bases[combo.Key]; ok {
			amt = b.Add(amt)
		}
		bases[combo.Key] = amt
	}
	return bases
}

// makeExciseTaxTotals builds one cac:TaxTotal per duty: category "Excise", the
// duty code as the scheme ID, name from the reason, TaxTypeCode from the caller.
func makeExciseTaxTotals(excises []exciseDuty, currency string) []TaxTotal {
	var totals []TaxTotal
	for _, e := range excises {
		amt := Amount{Value: e.amount.String(), CurrencyID: &currency}
		taxable := amt
		if e.base != nil {
			taxable = Amount{Value: e.base.String(), CurrencyID: &currency}
		}
		schemeID := schemeTaxScheme
		schemeAgencyID := agencyID
		typeAgencyID := agencyID
		listID := listTaxType
		scheme := &TaxScheme{
			ID: IDType{SchemeID: &schemeID, SchemeAgencyID: &schemeAgencyID, Value: e.scheme},
		}
		if e.typeCode != "" {
			scheme.TaxTypeCode = &IDType{ListAgencyID: &typeAgencyID, ListID: &listID, Value: e.typeCode}
		}
		if e.name != "" {
			name := e.name
			scheme.Name = &name
		}
		totals = append(totals, TaxTotal{
			TaxAmount: amt,
			TaxSubtotal: []TaxSubtotal{{
				TaxableAmount: taxable,
				TaxAmount:     amt,
				TaxCategory: TaxCategory{
					ID:        stampTaxCategoryID(&IDType{Value: taxCategoryExcise}),
					TaxScheme: scheme,
				},
			}},
		})
	}
	return totals
}

// exciseLineChargesFromTaxTotals is the inverse of makeExciseTaxTotals: each
// cac:TaxTotal/Excise subtotal becomes a bill.LineCharge (VAT subtotals ignored).
func exciseLineChargesFromTaxTotals(totals []TaxTotal) ([]*bill.LineCharge, error) {
	var charges []*bill.LineCharge
	for _, tt := range totals {
		for i := range tt.TaxSubtotal {
			st := &tt.TaxSubtotal[i]
			if st.TaxCategory.ID == nil || !isExciseCategoryID(st.TaxCategory.ID.Value) {
				continue
			}
			if st.TaxCategory.TaxScheme == nil {
				continue
			}
			amount, err := num.AmountFromString(ubl.NormalizeNumericString(st.TaxAmount.Value))
			if err != nil {
				return nil, err
			}
			ch := &bill.LineCharge{
				Key:    oioubl.ChargeKeyExcise,
				Ext:    dutyCodeExt(st.TaxCategory.TaxScheme.ID.Value),
				Amount: amount,
			}
			if st.TaxCategory.TaxScheme.Name != nil {
				ch.Reason = *st.TaxCategory.TaxScheme.Name
			}
			if st.TaxableAmount.Value != "" {
				base, err := num.AmountFromString(ubl.NormalizeNumericString(st.TaxableAmount.Value))
				if err != nil {
					return nil, err
				}
				ch.Base = &base
			}
			charges = append(charges, ch)
		}
	}
	return charges, nil
}

// baseKeyPart renders an optional base amount for use in a dedup map key,
// so two mirror keys differing only in a nil vs. zero base don't collide.
func baseKeyPart(base *num.Amount) string {
	if base == nil {
		return ""
	}
	return base.String()
}

// exciseMirrorKey keys a duty by code+amount+base, so a document-level
// TaxTotal/Excise entry can be matched against a line-level one it mirrors.
func exciseMirrorKey(dutyCode string, amount num.Amount, base *num.Amount) string {
	return dutyCode + "|" + amount.String() + "|" + baseKeyPart(base)
}

// lineExciseMirrors keys every line-level excise charge, so document-level
// TaxTotal/Excise entries that just mirror one can be told from genuine ones.
func lineExciseMirrors(lines []*bill.Line) map[string]bool {
	mirrors := make(map[string]bool)
	for _, l := range lines {
		for _, ch := range l.Charges {
			if chargeIsExcise(ch.Key) {
				mirrors[exciseMirrorKey(chargeDutyCode(ch.Ext), ch.Amount, ch.Base)] = true
			}
		}
	}
	return mirrors
}

// exciseChargesFromTaxTotals is exciseLineChargesFromTaxTotals's document-level
// analogue, also reading the duty's own TaxTypeCode back into its VAT combo.
// mirrors is skipped rather than double-counted as a genuine document charge.
func exciseChargesFromTaxTotals(totals []TaxTotal, mirrors map[string]bool) ([]*bill.Charge, error) {
	var charges []*bill.Charge
	for _, tt := range totals {
		for i := range tt.TaxSubtotal {
			st := &tt.TaxSubtotal[i]
			if st.TaxCategory.ID == nil || !isExciseCategoryID(st.TaxCategory.ID.Value) {
				continue
			}
			if st.TaxCategory.TaxScheme == nil {
				continue
			}
			amount, err := num.AmountFromString(ubl.NormalizeNumericString(st.TaxAmount.Value))
			if err != nil {
				return nil, err
			}
			var base *num.Amount
			if st.TaxableAmount.Value != "" {
				b, err := num.AmountFromString(ubl.NormalizeNumericString(st.TaxableAmount.Value))
				if err != nil {
					return nil, err
				}
				base = &b
			}
			if mirrors[exciseMirrorKey(st.TaxCategory.TaxScheme.ID.Value, amount, base)] {
				continue
			}
			ch := &bill.Charge{
				Key:    oioubl.ChargeKeyExcise,
				Ext:    dutyCodeExt(st.TaxCategory.TaxScheme.ID.Value),
				Amount: amount,
				Base:   base,
			}
			if st.TaxCategory.TaxScheme.Name != nil {
				ch.Reason = *st.TaxCategory.TaxScheme.Name
			}
			if tc := st.TaxCategory.TaxScheme.TaxTypeCode; tc != nil && tc.Value != "" {
				if key := goblVATKey(tc.Value); key != "" {
					ch.Taxes = tax.Set{{Category: tax.CategoryVAT, Key: key}}
				}
			}
			charges = append(charges, ch)
		}
	}
	return charges, nil
}

// goblAddExciseCharges parses the document's own cac:TaxTotal/Excise blocks,
// skipping any that just mirror a duty already parsed from a line.
func (ui *Invoice) goblAddExciseCharges(out *bill.Invoice) error {
	charges, err := exciseChargesFromTaxTotals(ui.TaxTotal, lineExciseMirrors(out.Lines))
	if err != nil {
		return err
	}
	out.Charges = append(out.Charges, charges...)
	return nil
}
