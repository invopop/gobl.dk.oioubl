package dkoioubl

import (
	"strings"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
)

// taxCategoryExcise is the taxcategoryid-1.1 category OIOUBL emits for a non-VAT
// excise duty (as a cac:TaxTotal, not a cac:AllowanceCharge).
const taxCategoryExcise = "Excise"

// exciseDuty is a duty resolved into the values OIOUBL needs.
type exciseDuty struct {
	scheme string
	name   string
	amount num.Amount
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
			out = append(out, exciseDuty{scheme: s, name: ch.Reason, amount: rescaleToCurrency(ch.Amount, currency)})
		}
	}
	for _, l := range inv.Lines {
		out = append(out, collectLineExcise(l, currency)...)
	}
	return out
}

// collectLineExcise gathers a single line's excise duties, mirrored as a
// line-level cac:TaxTotal so the wire records which line each duty belongs to.
func collectLineExcise(line *bill.Line, currency string) []exciseDuty {
	var out []exciseDuty
	for _, ch := range line.Charges {
		if s := chargeExciseScheme(ch.Key); s != "" {
			out = append(out, exciseDuty{scheme: s, name: ch.Reason, amount: rescaleToCurrency(ch.Amount, currency)})
		}
	}
	return out
}

// makeExciseTaxTotals builds one cac:TaxTotal per duty (category "Excise", the
// duty code in the scheme, name from the reason, TaxTypeCode from the amount sign).
func makeExciseTaxTotals(excises []exciseDuty, currency string) []TaxTotal {
	var totals []TaxTotal
	for _, e := range excises {
		amt := Amount{Value: e.amount.String(), CurrencyID: &currency}
		typeCode := taxCategoryStandardRated
		if e.amount.IsZero() {
			typeCode = taxCategoryZeroRated
		}
		schemeID := schemeTaxScheme
		schemeAgencyID := agencyID
		typeAgencyID := agencyID
		listID := listTaxType
		scheme := &TaxScheme{
			ID:          IDType{SchemeID: &schemeID, SchemeAgencyID: &schemeAgencyID, Value: e.scheme},
			TaxTypeCode: &IDType{ListAgencyID: &typeAgencyID, ListID: &listID, Value: typeCode},
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

// goblAddExciseCharges is a fallback: it attaches document-level excise to the
// first line only when no line carried its own block (handled in goblConvertLine).
func (ui *Invoice) goblAddExciseCharges(out *bill.Invoice) error {
	if len(out.Lines) == 0 {
		return nil
	}
	for _, l := range out.Lines {
		for _, ch := range l.Charges {
			if chargeExciseScheme(ch.Key) != "" {
				// A line already carried its excise; the document totals are its mirror.
				return nil
			}
		}
	}
	charges, err := exciseLineChargesFromTaxTotals(ui.TaxTotal)
	if err != nil {
		return err
	}
	out.Lines[0].Charges = append(out.Lines[0].Charges, charges...)
	return nil
}
