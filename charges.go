package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

// applyCharges drops excise-keyed charges (emitted as their own cac:TaxTotal
// instead) and fixes MultiplierFactorNumeric to OIOUBL's decimal-factor form (F-LIB228).
func (ui *Invoice) applyCharges(inv *bill.Invoice) {
	if len(ui.AllowanceCharge) == 0 {
		return
	}
	kept := make([]AllowanceCharge, 0, len(ui.AllowanceCharge))
	for i, ch := range inv.Charges {
		if chargeIsExcise(ch.Key) {
			continue
		}
		ac := ui.AllowanceCharge[i]
		applyAllowanceCharge(&ac, ch.Percent, ch.Taxes)
		kept = append(kept, ac)
	}
	for i, d := range inv.Discounts {
		ac := ui.AllowanceCharge[len(inv.Charges)+i]
		applyAllowanceCharge(&ac, d.Percent, d.Taxes)
		kept = append(kept, ac)
	}
	ui.AllowanceCharge = kept
}

// applyAllowanceCharge stamps MultiplierFactorNumeric and taxcategoryid;
// gobl.ubl's own AddCharges already handles the base amount.
func applyAllowanceCharge(ac *AllowanceCharge, pct *num.Percentage, taxes tax.Set) {
	if pct != nil {
		p := allowanceMultiplier(pct)
		ac.MultiplierFactorNumeric = &p
	}
	for i, t := range taxes {
		if i >= len(ac.TaxCategory) {
			break
		}
		if e := taxCategoryID(t.Key); e != "" {
			ac.TaxCategory[i].ID = &IDType{Value: e}
		}
	}
}

// allowanceMultiplier renders MultiplierFactorNumeric as a decimal factor (F-LIB228).
func allowanceMultiplier(pct *num.Percentage) string {
	return pct.Base().String()
}

// makeTaxCategory builds on gobl.ubl's tax-category builder, deriving the ID from the GOBL key instead.
func makeTaxCategory(taxes tax.Set) []*TaxCategory {
	cats := ubl.MakeTaxCategory(taxes)
	for i, t := range taxes {
		if e := taxCategoryID(t.Key); e != "" {
			cats[i].ID = &IDType{Value: e}
		}
	}
	return cats
}
