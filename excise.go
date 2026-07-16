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
// codes (3010-3671) that predate taxcategoryid-1.1's "Excise" value; zero
// occurrences in the 428-file ERST real corpus, but treated as excise on parse
// if one ever shows up, since the alternative is silently dropping a real duty.
const (
	legacyExciseCategoryMin = 3010
	legacyExciseCategoryMax = 3671
)

// isExciseCategoryID reports whether a taxcategoryid-1.1 value is either
// OIOUBL's own "Excise" or a legacy UNCL5305 numeric duty code. Only "Excise"
// is ever emitted outbound -- this widening is inbound-only and lossy (the
// specific legacy code is discarded).
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
			out = append(out, exciseDuty{
				scheme:   chargeDutyCode(ch.Ext),
				name:     ch.Reason,
				amount:   rescaleToCurrency(ch.Amount, currency),
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

// exciseVATBases sums, per VAT key, the base the document-level excise charges'
// own VAT combos contribute to the invoice's tax totals (in the invoice
// currency), so addTotals can recognize a VAT rate row owed entirely to excise.
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
				TaxableAmount: amt,
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
			charges = append(charges, ch)
		}
	}
	return charges, nil
}

// exciseChargesFromTaxTotals is the document-level analogue of
// exciseLineChargesFromTaxTotals, reading the duty's own TaxTypeCode back into
// the charge's VAT combo.
func exciseChargesFromTaxTotals(totals []TaxTotal) ([]*bill.Charge, error) {
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
			ch := &bill.Charge{
				Key:    oioubl.ChargeKeyExcise,
				Ext:    dutyCodeExt(st.TaxCategory.TaxScheme.ID.Value),
				Amount: amount,
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

// goblAddExciseCharges parses the document's own cac:TaxTotal/Excise blocks
// (ui.TaxTotal, which by XML structure never includes a line's nested
// blocks — those are parsed separately in goblConvertLine) into genuine
// bill.Charge entries, unless a line already carries its own excise block, in
// which case the document totals are just that duty's mirror and are skipped.
func (ui *Invoice) goblAddExciseCharges(out *bill.Invoice) error {
	for _, l := range out.Lines {
		for _, ch := range l.Charges {
			if chargeIsExcise(ch.Key) {
				return nil
			}
		}
	}
	charges, err := exciseChargesFromTaxTotals(ui.TaxTotal)
	if err != nil {
		return err
	}
	out.Charges = append(out.Charges, charges...)
	return nil
}
