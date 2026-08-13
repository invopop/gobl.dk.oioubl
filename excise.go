package oioubl

import (
	"github.com/invopop/gobl.dk.oioubl/addon"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

// taxCategoryExcise is the taxcategoryid-1.1 category OIOUBL emits for a non-VAT
// excise duty (as a cac:TaxTotal, not a cac:AllowanceCharge).
const taxCategoryExcise = "Excise"

type exciseDuty struct {
	scheme   string
	name     string
	amount   num.Amount
	base     *num.Amount
	percent  *num.Percentage
	typeCode string
}

func chargeIsExcise(key cbc.Key) bool {
	return key == addon.ChargeKeyExcise
}

// chargeDutyCode returns an excise charge's SKAT duty code (OIOUBL taxschemeid, e.g. "16").
func chargeDutyCode(ext tax.Extensions) string {
	return ext.Get(addon.ExtKeyDutyCode).String()
}

func collectExcise(inv *bill.Invoice, currency string) []exciseDuty {
	var out []exciseDuty
	for _, ch := range inv.Charges {
		if chargeIsExcise(ch.Key) {
			out = append(out, exciseDuty{
				scheme:   chargeDutyCode(ch.Ext),
				name:     ch.Reason,
				amount:   ch.Amount,
				base:     ch.Base,
				percent:  ch.Percent,
				typeCode: chargeVATTypeCode(ch),
			})
		}
	}
	for _, l := range inv.Lines {
		out = append(out, collectLineExcise(l, currency)...)
	}
	return out
}

// collectLineExcise gathers a line's excise duties. Each one stays on its own
// line, so it is clear what was charged where.
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

// lineVATTypeCode: a line's duties inherit their taxtypecode from the line's own VAT category (OIOUBL Skat guideline).
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
// contribute to the invoice's tax totals, so buildTotals can spot excise-only rows.
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

// makeExciseTaxTotals builds one cac:TaxTotal per duty scheme (code), grouping
// same-code duties into shared TaxSubtotal entries: OIOUBL forbids the same
// duty code from appearing in more than one TaxTotal class (G27 3.5). The
// second return is the total duty across every scheme.
func makeExciseTaxTotals(excises []exciseDuty, currency string) ([]ubl.TaxTotal, num.Amount) {
	var order []string
	subtotals := make(map[string][]ubl.TaxSubtotal)
	sums := make(map[string]num.Amount)

	for _, e := range excises {
		amt := ubl.Amount{Value: e.amount.String(), CurrencyID: &currency}
		taxable := amt
		if e.base != nil {
			taxable = ubl.Amount{Value: e.base.String(), CurrencyID: &currency}
		}
		schemeID := schemeTaxScheme
		schemeAgencyID := agencyID
		typeAgencyID := agencyID
		listID := codelistTaxType
		scheme := &ubl.TaxScheme{
			ID: ubl.IDType{SchemeID: &schemeID, SchemeAgencyID: &schemeAgencyID, Value: e.scheme},
		}
		if e.typeCode != "" {
			scheme.TaxTypeCode = &ubl.IDType{ListAgencyID: &typeAgencyID, ListID: &listID, Value: e.typeCode}
		}
		if e.name != "" {
			scheme.Name = ptr(e.name)
		}

		if _, ok := subtotals[e.scheme]; !ok {
			order = append(order, e.scheme)
			sums[e.scheme] = e.amount
		} else {
			sums[e.scheme] = sums[e.scheme].Add(e.amount.Rescale(sums[e.scheme].Exp()))
		}
		cat := ubl.TaxCategory{
			ID:        stampTaxCategoryID(&ubl.IDType{Value: taxCategoryExcise}),
			TaxScheme: scheme,
		}
		if e.percent != nil {
			cat.Percent = ptr(e.percent.StringWithoutSymbol())
		}
		subtotals[e.scheme] = append(subtotals[e.scheme], ubl.TaxSubtotal{
			TaxableAmount: taxable,
			TaxAmount:     amt,
			TaxCategory:   cat,
		})
	}

	var totals []ubl.TaxTotal
	var total num.Amount
	for i, scheme := range order {
		if i == 0 {
			total = sums[scheme]
		} else {
			total = total.Add(sums[scheme].Rescale(total.Exp()))
		}
		totals = append(totals, ubl.TaxTotal{
			TaxAmount:   ubl.Amount{Value: sums[scheme].String(), CurrencyID: &currency},
			TaxSubtotal: subtotals[scheme],
		})
	}
	return totals, total
}

// isExciseOnlyRate reports whether a VAT rate row is owed entirely to excise, so it gets no subtotal of its own (F-LIB404).
func isExciseOnlyRate(cat *tax.CategoryTotal, r *tax.RateTotal, exciseBases map[cbc.Key]num.Amount) bool {
	if cat.Code != tax.CategoryVAT || !r.Amount.IsZero() {
		return false
	}
	base, ok := exciseBases[r.Key]
	return ok && r.Base.Compare(base.Rescale(r.Base.Exp())) == 0
}
