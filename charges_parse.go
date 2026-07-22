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

// goblAddCharges parses the document's own cac:AllowanceCharge entries. Per
// OIOUBL's guideline (G17 3.5), only header-level AllowanceCharge is real
// money -- line-level entries never reach this far (see applyLineAllowanceNotes).
func (ui *Invoice) goblAddCharges(out *bill.Invoice) error {
	var charges []*bill.Charge
	var discounts []*bill.Discount

	taxCategoryMap := (*ubl.Invoice)(ui).BuildTaxCategoryMap()

	for _, allowanceCharge := range ui.AllowanceCharge {
		if allowanceCharge.ChargeIndicator {
			charge, err := goblCharge(&allowanceCharge, taxCategoryMap)
			if err != nil {
				return err
			}
			charges = append(charges, charge)
		} else {
			discount, err := goblDiscount(&allowanceCharge, taxCategoryMap)
			if err != nil {
				return err
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

// goblAllowancePercent reads OIOUBL's decimal factor (0.05 = 5%, F-LIB228);
// gobl.ubl's shared parser reads EN 16931's percentage form (5 = 5%) instead.
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

// parsedAllowanceCharge holds fields shared by a parsed document-level charge/discount.
type parsedAllowanceCharge struct {
	Reason  string
	Amount  num.Amount
	Base    *num.Amount
	Percent *num.Percentage
	Ext     tax.Extensions
	Taxes   tax.Set
}

// parseAllowanceCharge reads a document-level charge/discount's shared fields
// (OIOUBL decimal-factor percent, F-LIB228) and its tax category.
func parseAllowanceCharge(ac *AllowanceCharge, extKey cbc.Key, taxCategoryMap map[string]*ubl.TaxCategoryInfo) (parsedAllowanceCharge, error) {
	var out parsedAllowanceCharge
	if ac.AllowanceChargeReason != nil {
		out.Reason = *ac.AllowanceChargeReason
	}
	if ac.Amount.Value != "" {
		a, err := num.AmountFromString(ubl.NormalizeNumericString(ac.Amount.Value))
		if err != nil {
			return out, err
		}
		out.Amount = a
	}
	if ac.AllowanceChargeReasonCode != nil {
		out.Ext = tax.ExtensionsOf(cbc.CodeMap{
			extKey: cbc.Code(*ac.AllowanceChargeReasonCode),
		})
	}
	if ac.BaseAmount != nil {
		b, err := num.AmountFromString(ubl.NormalizeNumericString(ac.BaseAmount.Value))
		if err != nil {
			return out, err
		}
		out.Base = &b
	}
	pct, err := goblAllowancePercent(ac)
	if err != nil {
		return out, err
	}
	if pct != nil {
		out.Percent = pct

		if ac.BaseAmount != nil {
			base, err := num.AmountFromString(ubl.NormalizeNumericString(ac.BaseAmount.Value))
			if err != nil {
				return out, err
			}
			out.Base = &base
		}
	}
	if len(ac.TaxCategory) > 0 && ac.TaxCategory[0].TaxScheme != nil {
		out.Taxes = tax.Set{
			{
				Category: goblTaxSchemeCategory(ac.TaxCategory[0].TaxScheme.ID.Value),
			},
		}

		if ac.TaxCategory[0].ID != nil {
			out.Taxes[0].Ext = out.Taxes[0].Ext.Set(untdid.ExtKeyTaxCategory, goblTaxCategoryCode(ac.TaxCategory[0].ID.Value))

			// Look up exemption code from TaxTotal
			key := ubl.BuildTaxCategoryKey(ac.TaxCategory[0].TaxScheme.ID.Value, ac.TaxCategory[0].ID.Value, ac.TaxCategory[0].Percent)
			if info, ok := taxCategoryMap[key]; ok && info.ExemptionReasonCode != "" {
				out.Taxes[0].Ext = out.Taxes[0].Ext.Set(cef.ExtKeyVATEX, cbc.Code(info.ExemptionReasonCode))
			}
		}

		if ac.TaxCategory[0].Percent != nil {
			percent := ubl.NormalizeNumericString(*ac.TaxCategory[0].Percent)
			if !strings.HasSuffix(percent, "%") {
				percent += "%"
			}
			p, err := num.PercentageFromString(percent)
			if err != nil {
				return out, err
			}

			// Skip 0% unless zero-rated, so GOBL doesn't normalize exempt/reverse-charge
			// to "zero"; compare via goblTaxCategoryCode to catch the "ZeroRated" wire value.
			if !p.IsZero() || (ac.TaxCategory[0].ID != nil && goblTaxCategoryCode(ac.TaxCategory[0].ID.Value) == "Z") {
				out.Taxes[0].Percent = &p
			}
		}
	}
	return out, nil
}

func goblCharge(ac *AllowanceCharge, taxCategoryMap map[string]*ubl.TaxCategoryInfo) (*bill.Charge, error) {
	p, err := parseAllowanceCharge(ac, untdid.ExtKeyCharge, taxCategoryMap)
	if err != nil {
		return nil, err
	}
	return &bill.Charge{
		Reason:  p.Reason,
		Amount:  p.Amount,
		Base:    p.Base,
		Percent: p.Percent,
		Ext:     p.Ext,
		Taxes:   p.Taxes,
	}, nil
}

func goblDiscount(ac *AllowanceCharge, taxCategoryMap map[string]*ubl.TaxCategoryInfo) (*bill.Discount, error) {
	p, err := parseAllowanceCharge(ac, untdid.ExtKeyAllowance, taxCategoryMap)
	if err != nil {
		return nil, err
	}
	return &bill.Discount{
		Reason:  p.Reason,
		Amount:  p.Amount,
		Base:    p.Base,
		Percent: p.Percent,
		Ext:     p.Ext,
		Taxes:   p.Taxes,
	}, nil
}
