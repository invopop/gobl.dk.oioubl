package dkoioubl

import (
	"strings"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

// taxCategoryExcise is the taxcategoryid-1.1 category OIOUBL emits for a non-VAT
// excise duty (as a cac:TaxTotal, not a cac:AllowanceCharge).
const taxCategoryExcise = "Excise"

// exciseDuty is a duty resolved into the values OIOUBL needs.
type exciseDuty struct {
	scheme string
	name   string
	amount num.Amount
	// typeCode is the duty's own taxtypecode-1.1/-1.2 value: read from the
	// charge's own VAT combo for a document-level duty (stated explicitly,
	// since it diverges from any one line's own category by definition), or
	// inherited from the line's own VAT category for a line-level duty
	// (never independently stated).
	typeCode string
}

// chargeExciseScheme returns the taxschemeid duty code an all-digit charge Key
// carries (e.g. "16"), or "" for an ordinary charge; a zero-padded "09" → "9".
func chargeExciseScheme(key cbc.Key) string {
	s := key.String()
	if s == "" || !isAllDigits(s) {
		return ""
	}
	if code := strings.TrimLeft(s, "0"); code != "" {
		return code
	}
	return "0"
}

// exciseSchemeKey builds the charge Key for an OIOUBL taxschemeid duty code,
// zero-padding a single digit so it is a valid cbc.Key.
func exciseSchemeKey(code string) cbc.Key {
	if len(code) == 1 {
		return cbc.Key("0" + code)
	}
	return cbc.Key(code)
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// collectExcise gathers every excise duty across document- and line-level charges.
func collectExcise(inv *bill.Invoice, currency string) []exciseDuty {
	var out []exciseDuty
	for _, ch := range inv.Charges {
		if s := chargeExciseScheme(ch.Key); s != "" {
			out = append(out, exciseDuty{
				scheme:   s,
				name:     ch.Reason,
				amount:   rescaleToCurrency(ch.Amount, currency),
				typeCode: chargeVATTypeCode(ch),
			})
		}
	}
	for _, l := range inv.Lines {
		out = append(out, collectLineExcise(l, currency)...)
	}
	return out
}

// collectLineExcise gathers a single line's excise duties, mirrored as a
// line-level cac:TaxTotal so the wire records which line each duty belongs to.
// A line-level duty's TaxTypeCode is always inherited from the line's own VAT
// category (never independently stated — see lineVATTypeCode).
func collectLineExcise(line *bill.Line, currency string) []exciseDuty {
	var out []exciseDuty
	typeCode := lineVATTypeCode(line)
	for _, ch := range line.Charges {
		if s := chargeExciseScheme(ch.Key); s != "" {
			out = append(out, exciseDuty{
				scheme:   s,
				name:     ch.Reason,
				amount:   rescaleToCurrency(ch.Amount, currency),
				typeCode: typeCode,
			})
		}
	}
	return out
}

// chargeVATTypeCode resolves a document-level charge's own VAT combo into the
// OIOUBL taxtypecode-1.1/-1.2 value, mirroring lineVATTypeCode: a duty is
// document-level precisely because its VAT type diverges from any one line's
// own category, so the charge states it in its own taxes.
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

// lineVATTypeCode resolves a line's own VAT category into the OIOUBL
// taxtypecode-1.1/-1.2 value, reusing the same mapping (taxCategoryID) used
// for the line's own cac:TaxCategory/ID — the OIOUBL Skat guideline requires
// a line-level excise duty to inherit this rather than state it independently.
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
func exciseVATBases(inv *bill.Invoice, currency string) map[cbc.Key]num.Amount {
	bases := make(map[cbc.Key]num.Amount)
	for _, ch := range inv.Charges {
		if chargeExciseScheme(ch.Key) == "" {
			continue
		}
		combo := ch.Taxes.Get(tax.CategoryVAT)
		if combo == nil {
			continue
		}
		amt := rescaleToCurrency(ch.Amount, currency)
		if b, ok := bases[combo.Key]; ok {
			amt = b.Add(amt)
		}
		bases[combo.Key] = amt
	}
	return bases
}

// makeExciseTaxTotals builds one cac:TaxTotal per duty (category "Excise", the
// duty code in the scheme, name from the reason, TaxTypeCode resolved by the
// caller — from the charge's own VAT combo for a document-level duty, inherited
// from the line's own VAT category for a line-level one; see exciseDuty.typeCode).
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
			if st.TaxCategory.ID == nil || st.TaxCategory.ID.Value != taxCategoryExcise {
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
				Key:    exciseSchemeKey(st.TaxCategory.TaxScheme.ID.Value),
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
// exciseLineChargesFromTaxTotals: each cac:TaxTotal/Excise subtotal becomes a
// genuine bill.Charge, with the duty's own TaxTypeCode (only ever stated
// because it diverges from a line's own VAT category) read back into the
// charge's own VAT combo.
func exciseChargesFromTaxTotals(totals []TaxTotal) ([]*bill.Charge, error) {
	var charges []*bill.Charge
	for _, tt := range totals {
		for i := range tt.TaxSubtotal {
			st := &tt.TaxSubtotal[i]
			if st.TaxCategory.ID == nil || st.TaxCategory.ID.Value != taxCategoryExcise {
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
				Key:    exciseSchemeKey(st.TaxCategory.TaxScheme.ID.Value),
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
			if chargeExciseScheme(ch.Key) != "" {
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
