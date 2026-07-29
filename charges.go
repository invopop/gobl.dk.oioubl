package oioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

// applyCharges drops excise-keyed charges (emitted as their own cac:TaxTotal
// instead) and fixes MultiplierFactorNumeric to OIOUBL's decimal-factor form (F-LIB228).
func (ui *Invoice) applyCharges(inv *bill.Invoice) {
	if len(ui.AllowanceCharge) == 0 {
		return
	}
	ccy := inv.Currency.String()
	var sum *num.Amount
	if inv.Totals != nil {
		sum = &inv.Totals.Sum
	}
	kept := make([]ubl.AllowanceCharge, 0, len(ui.AllowanceCharge))
	for i, ch := range inv.Charges {
		if chargeIsExcise(ch.Key) {
			continue
		}
		ac := ui.AllowanceCharge[i]
		applyAllowanceCharge(&ac, ch.Percent, ch.Taxes)
		if ba := chargeBaseAmount(ch.Percent, ch.Base, sum, ccy); ba != nil {
			ac.BaseAmount = ba
		}
		kept = append(kept, ac)
	}
	for i, d := range inv.Discounts {
		ac := ui.AllowanceCharge[len(inv.Charges)+i]
		applyAllowanceCharge(&ac, d.Percent, d.Taxes)
		if ba := chargeBaseAmount(d.Percent, d.Base, sum, ccy); ba != nil {
			ac.BaseAmount = ba
		}
		kept = append(kept, ac)
	}
	ui.AllowanceCharge = kept
}

// chargeBaseAmount is the amount a percentage is taken off: the charge's own
// base if it has one, otherwise the invoice total.
func chargeBaseAmount(pct *num.Percentage, base, fallback *num.Amount, ccy string) *ubl.Amount {
	if pct == nil {
		return nil
	}
	b := fallback
	if base != nil {
		b = base
	}
	if b == nil {
		return nil
	}
	return &ubl.Amount{Value: rescaleToCurrency(*b, ccy).String(), CurrencyID: &ccy}
}

// applyAllowanceCharge stamps MultiplierFactorNumeric and taxcategoryid.
func applyAllowanceCharge(ac *ubl.AllowanceCharge, pct *num.Percentage, taxes tax.Set) {
	if pct != nil {
		ac.MultiplierFactorNumeric = ptr(allowanceMultiplier(pct))
	}
	for i, t := range taxes {
		if i >= len(ac.TaxCategory) {
			break
		}
		if e := taxCategoryID(t.Key); e != "" {
			ac.TaxCategory[i].ID = &ubl.IDType{Value: e}
		}
	}
}

// allowanceMultiplier writes a percentage the way OIOUBL wants it: as a
// fraction, so 25% goes out as 0.25 and amount = base x fraction (F-LIB228).
func allowanceMultiplier(pct *num.Percentage) string {
	return pct.Base().String()
}

// makeTaxCategory mirrors the base's tax-category builder, deriving the ID from the GOBL key instead.
func makeTaxCategory(taxes tax.Set) []*ubl.TaxCategory {
	cats := make([]*ubl.TaxCategory, 0, len(taxes))
	for _, t := range taxes {
		cats = append(cats, &ubl.TaxCategory{
			ID:        taxCategoryCode(t),
			Percent:   taxCategoryPercent(t),
			TaxScheme: &ubl.TaxScheme{ID: ubl.IDType{Value: t.Category.String()}},
		})
	}
	return cats
}

// taxCategoryCode prefers the OIOUBL code the GOBL key maps to, falling back to
// the UNTDID category the base would have used.
func taxCategoryCode(t *tax.Combo) *ubl.IDType {
	for _, code := range []string{taxCategoryID(t.Key), t.Ext.Get(untdid.ExtKeyTaxCategory).String()} {
		if code != "" {
			return &ubl.IDType{Value: code}
		}
	}
	return nil
}

// taxCategoryPercent is required on every category except UNTDID "O", which is
// outside the scope of tax and so has no rate to state.
func taxCategoryPercent(t *tax.Combo) *string {
	if t.Percent != nil {
		return ptr(t.Percent.StringWithoutSymbol())
	}
	if t.Ext.Get(untdid.ExtKeyTaxCategory).String() == string(en16931.TaxCategoryOutsideScope) {
		return nil
	}
	return ptr("0")
}
