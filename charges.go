package dkoioubl

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

func (ui *Invoice) addCharges(inv *bill.Invoice) {
	if inv.Charges == nil && inv.Discounts == nil {
		return
	}
	// Use invoice sum (before discounts) as base amount for percentage calculations
	baseAmount := inv.Totals.Sum
	for _, ch := range inv.Charges {
		// OIOUBL emits an excise duty (a charge whose Key is a taxschemeid code) as
		// a cac:TaxTotal/Excise subtotal built in addTotals, not as a charge.
		if chargeExciseScheme(ch.Key) != "" {
			continue
		}
		ui.AllowanceCharge = append(ui.AllowanceCharge, makeCharge(ch, string(inv.Currency), baseAmount))
	}
	for _, d := range inv.Discounts {
		ui.AllowanceCharge = append(ui.AllowanceCharge, makeDiscount(d, string(inv.Currency), baseAmount))
	}
}

// allowanceMultiplier renders MultiplierFactorNumeric as the decimal factor
// OIOUBL F-LIB228 requires (Amount = BaseAmount × factor).
func allowanceMultiplier(pct *num.Percentage) string {
	return pct.Base().String()
}

func makeCharge(ch *bill.Charge, ccy string, baseAmount num.Amount) AllowanceCharge {
	c := AllowanceCharge{
		ChargeIndicator: true,
		Amount: Amount{
			Value:      ch.Amount.String(),
			CurrencyID: &ccy,
		},
	}
	if ch.Reason != "" {
		c.AllowanceChargeReason = &ch.Reason
	}
	e := ch.Ext.Get(untdid.ExtKeyCharge).String()
	if e != "" {
		c.AllowanceChargeReasonCode = &e
	}
	if ch.Percent != nil {
		p := allowanceMultiplier(ch.Percent)
		c.MultiplierFactorNumeric = &p
		c.BaseAmount = &Amount{
			Value:      baseAmount.String(),
			CurrencyID: &ccy,
		}
	}
	if ch.Taxes != nil {
		c.TaxCategory = makeTaxCategory(ch.Taxes)
	}

	return c
}

func makeDiscount(d *bill.Discount, ccy string, baseAmount num.Amount) AllowanceCharge {
	c := AllowanceCharge{
		ChargeIndicator: false,
		Amount: Amount{
			Value:      d.Amount.String(),
			CurrencyID: &ccy,
		},
	}
	if d.Reason != "" {
		c.AllowanceChargeReason = &d.Reason
	}
	e := d.Ext.Get(untdid.ExtKeyAllowance).String()
	if e != "" {
		c.AllowanceChargeReasonCode = &e
	}
	if d.Percent != nil {
		p := allowanceMultiplier(d.Percent)
		c.MultiplierFactorNumeric = &p
		c.BaseAmount = &Amount{
			Value:      baseAmount.String(),
			CurrencyID: &ccy,
		}
	}
	if d.Taxes != nil {
		c.TaxCategory = makeTaxCategory(d.Taxes)
	}

	return c
}

func makeTaxCategory(taxes tax.Set) []*TaxCategory {
	set := []*TaxCategory{}
	for _, t := range taxes {
		category := TaxCategory{}
		category.TaxScheme = &TaxScheme{ID: IDType{Value: t.Category.String()}}

		// OIOUBL emits its own taxcategoryid-1.1 value (StandardRated/…).
		if e := taxCategoryID(t.Key); e != "" {
			category.ID = &IDType{Value: e}
		}

		// Set percent: required unless category is "O" (outside scope)
		if t.Percent != nil {
			p := t.Percent.StringWithoutSymbol()
			category.Percent = &p
		} else if category.ID == nil || category.ID.Value != "O" {
			zero := "0"
			category.Percent = &zero
		}

		set = append(set, &category)
	}
	return set
}
