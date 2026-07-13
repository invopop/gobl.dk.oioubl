package dkoioubl

import (
	"strings"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/cef"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/num"
	"github.com/invopop/gobl/tax"
)

func (ui *Invoice) goblAddCharges(out *bill.Invoice) error {
	var charges []*bill.Charge
	var discounts []*bill.Discount

	taxCategoryMap := ui.buildTaxCategoryMap()

	for _, allowanceCharge := range ui.AllowanceCharge {
		if allowanceCharge.ChargeIndicator {
			charge, err := goblCharge(&allowanceCharge, taxCategoryMap)
			if err != nil {
				return err
			}
			if charges == nil {
				charges = make([]*bill.Charge, 0)
			}
			charges = append(charges, charge)
		} else {
			discount, err := goblDiscount(&allowanceCharge, taxCategoryMap)
			if err != nil {
				return err
			}
			if discounts == nil {
				discounts = make([]*bill.Discount, 0)
			}
			discounts = append(discounts, discount)
		}
	}
	if charges != nil {
		out.Charges = charges
	}
	if discounts != nil {
		out.Discounts = discounts
	}
	return nil
}

// goblAllowancePercent reads the decimal MultiplierFactorNumeric (0.05 = 5%) into a GOBL percentage.
func goblAllowancePercent(ac *AllowanceCharge) (*num.Percentage, error) {
	if ac.MultiplierFactorNumeric == nil {
		return nil, nil
	}
	p, err := num.PercentageFromString(ubl.NormalizeNumericString(*ac.MultiplierFactorNumeric))
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func goblCharge(ac *AllowanceCharge, taxCategoryMap map[string]*taxCategoryInfo) (*bill.Charge, error) {
	ch := &bill.Charge{}
	if ac.AllowanceChargeReason != nil {
		ch.Reason = *ac.AllowanceChargeReason
	}
	if ac.Amount.Value != "" {
		a, err := num.AmountFromString(ubl.NormalizeNumericString(ac.Amount.Value))
		if err != nil {
			return nil, err
		}
		ch.Amount = a
	}
	if ac.AllowanceChargeReasonCode != nil {
		ch.Ext = tax.ExtensionsOf(cbc.CodeMap{
			untdid.ExtKeyCharge: cbc.Code(*ac.AllowanceChargeReasonCode),
		})
	}
	if ac.BaseAmount != nil {
		b, err := num.AmountFromString(ubl.NormalizeNumericString(ac.BaseAmount.Value))
		if err != nil {
			return nil, err
		}
		ch.Base = &b
	}
	pct, err := goblAllowancePercent(ac)
	if err != nil {
		return nil, err
	}
	if pct != nil {
		ch.Percent = pct

		if ac.BaseAmount != nil {
			base, err := num.AmountFromString(ubl.NormalizeNumericString(ac.BaseAmount.Value))
			if err != nil {
				return nil, err
			}
			ch.Base = &base
		}
	}
	if len(ac.TaxCategory) > 0 && ac.TaxCategory[0].TaxScheme != nil {
		ch.Taxes = tax.Set{
			{
				Category: goblTaxSchemeCategory(ac.TaxCategory[0].TaxScheme.ID.Value),
			},
		}

		if ac.TaxCategory[0].ID != nil {
			ch.Taxes[0].Ext = ch.Taxes[0].Ext.Set(untdid.ExtKeyTaxCategory, goblTaxCategoryCode(ac.TaxCategory[0].ID.Value))

			// Look up exemption code from TaxTotal
			key := ubl.BuildTaxCategoryKey(ac.TaxCategory[0].TaxScheme.ID.Value, ac.TaxCategory[0].ID.Value, ac.TaxCategory[0].Percent)
			if info, ok := taxCategoryMap[key]; ok && info.exemptionReasonCode != "" {
				ch.Taxes[0].Ext = ch.Taxes[0].Ext.Set(cef.ExtKeyVATEX, cbc.Code(info.exemptionReasonCode))
			}
		}

		if ac.TaxCategory[0].Percent != nil {
			percent := ubl.NormalizeNumericString(*ac.TaxCategory[0].Percent)
			if !strings.HasSuffix(percent, "%") {
				percent += "%"
			}
			p, err := num.PercentageFromString(percent)
			if err != nil {
				return nil, err
			}

			// Skip 0% unless zero-rated ("Z"), so GOBL doesn't normalize exempt/reverse-charge to "zero".
			if !p.IsZero() || (ac.TaxCategory[0].ID != nil && ac.TaxCategory[0].ID.Value == "Z") {
				ch.Taxes[0].Percent = &p
			}
		}
	}
	return ch, nil
}

func goblDiscount(ac *AllowanceCharge, taxCategoryMap map[string]*taxCategoryInfo) (*bill.Discount, error) {
	d := &bill.Discount{}
	if ac.AllowanceChargeReason != nil {
		d.Reason = *ac.AllowanceChargeReason
	}
	if ac.Amount.Value != "" {
		a, err := num.AmountFromString(ubl.NormalizeNumericString(ac.Amount.Value))
		if err != nil {
			return nil, err
		}
		d.Amount = a
	}
	if ac.AllowanceChargeReasonCode != nil {
		d.Ext = tax.ExtensionsOf(cbc.CodeMap{
			untdid.ExtKeyAllowance: cbc.Code(*ac.AllowanceChargeReasonCode),
		})
	}
	if ac.BaseAmount != nil {
		b, err := num.AmountFromString(ubl.NormalizeNumericString(ac.BaseAmount.Value))
		if err != nil {
			return nil, err
		}
		d.Base = &b
	}
	pct, err := goblAllowancePercent(ac)
	if err != nil {
		return nil, err
	}
	if pct != nil {
		d.Percent = pct

		if ac.BaseAmount != nil {
			base, err := num.AmountFromString(ubl.NormalizeNumericString(ac.BaseAmount.Value))
			if err != nil {
				return nil, err
			}
			d.Base = &base
		}
	}
	if len(ac.TaxCategory) > 0 && ac.TaxCategory[0].TaxScheme != nil {
		d.Taxes = tax.Set{
			{
				Category: goblTaxSchemeCategory(ac.TaxCategory[0].TaxScheme.ID.Value),
			},
		}

		if ac.TaxCategory[0].ID != nil {
			d.Taxes[0].Ext = d.Taxes[0].Ext.Set(untdid.ExtKeyTaxCategory, goblTaxCategoryCode(ac.TaxCategory[0].ID.Value))

			// Look up exemption code from TaxTotal
			key := ubl.BuildTaxCategoryKey(ac.TaxCategory[0].TaxScheme.ID.Value, ac.TaxCategory[0].ID.Value, ac.TaxCategory[0].Percent)
			if info, ok := taxCategoryMap[key]; ok && info.exemptionReasonCode != "" {
				d.Taxes[0].Ext = d.Taxes[0].Ext.Set(cef.ExtKeyVATEX, cbc.Code(info.exemptionReasonCode))
			}
		}

		if ac.TaxCategory[0].Percent != nil {
			percentStr := ubl.NormalizeNumericString(*ac.TaxCategory[0].Percent)
			if !strings.HasSuffix(percentStr, "%") {
				percentStr += "%"
			}
			percent, err := num.PercentageFromString(percentStr)
			if err != nil {
				return nil, err
			}

			// Skip 0% unless zero-rated ("Z"), so GOBL doesn't normalize exempt/reverse-charge to "zero".
			if !percent.IsZero() || (ac.TaxCategory[0].ID != nil && ac.TaxCategory[0].ID.Value == "Z") {
				d.Taxes[0].Percent = &percent
			}
		}
	}
	return d, nil
}

func goblLineCharge(ac *AllowanceCharge) (*bill.LineCharge, error) {
	amount, err := num.AmountFromString(ubl.NormalizeNumericString(ac.Amount.Value))
	if err != nil {
		return nil, err
	}
	ch := &bill.LineCharge{
		Amount: amount,
	}
	if ac.AllowanceChargeReasonCode != nil {
		ch.Ext = tax.ExtensionsOf(cbc.CodeMap{
			untdid.ExtKeyCharge: cbc.Code(*ac.AllowanceChargeReasonCode),
		})
	}
	if ac.AllowanceChargeReason != nil {
		ch.Reason = *ac.AllowanceChargeReason
	}
	pct, err := goblAllowancePercent(ac)
	if err != nil {
		return nil, err
	}
	if pct != nil {
		ch.Percent = pct

		if ac.BaseAmount != nil {
			base, err := num.AmountFromString(ubl.NormalizeNumericString(ac.BaseAmount.Value))
			if err != nil {
				return nil, err
			}
			ch.Base = &base
		}
	}
	return ch, nil
}

func goblLineDiscount(ac *AllowanceCharge) (*bill.LineDiscount, error) {
	a, err := num.AmountFromString(ubl.NormalizeNumericString(ac.Amount.Value))
	if err != nil {
		return nil, err
	}
	d := &bill.LineDiscount{
		Amount: a,
	}
	if ac.AllowanceChargeReasonCode != nil {
		d.Ext = tax.ExtensionsOf(cbc.CodeMap{
			untdid.ExtKeyAllowance: cbc.Code(*ac.AllowanceChargeReasonCode),
		})
	}
	if ac.AllowanceChargeReason != nil {
		d.Reason = *ac.AllowanceChargeReason
	}
	pct, err := goblAllowancePercent(ac)
	if err != nil {
		return nil, err
	}
	if pct != nil {
		d.Percent = pct

		if ac.BaseAmount != nil {
			base, err := num.AmountFromString(ubl.NormalizeNumericString(ac.BaseAmount.Value))
			if err != nil {
				return nil, err
			}
			d.Base = &base
		}
	}
	return d, nil
}
