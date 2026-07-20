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

// goblAddCharges parses the document's own cac:AllowanceCharge entries,
// skipping any that just mirror a charge/discount already parsed from a line.
func (ui *Invoice) goblAddCharges(out *bill.Invoice) error {
	var charges []*bill.Charge
	var discounts []*bill.Discount

	taxCategoryMap := (*ubl.Invoice)(ui).BuildTaxCategoryMap()
	chargeMirrors := lineChargeMirrors(out.Lines)
	discountMirrors := lineDiscountMirrors(out.Lines)

	for _, allowanceCharge := range ui.AllowanceCharge {
		if allowanceCharge.ChargeIndicator {
			charge, err := goblCharge(&allowanceCharge, taxCategoryMap)
			if err != nil {
				return err
			}
			if key := chargeMirrorKey(charge.Reason, charge.Amount, charge.Base); chargeMirrors[key] > 0 {
				chargeMirrors[key]--
				continue
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
			if key := chargeMirrorKey(discount.Reason, discount.Amount, discount.Base); discountMirrors[key] > 0 {
				discountMirrors[key]--
				continue
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

// chargeMirrorKey keys a charge/discount by reason+amount+base, so a
// document-level entry can be matched against a line-level one it mirrors.
func chargeMirrorKey(reason string, amount num.Amount, base *num.Amount) string {
	return reason + "|" + amount.String() + "|" + baseKeyPart(base)
}

// lineChargeMirrors counts line-level charges, so goblAddCharges can drop
// document-level entries that just mirror one already captured at the line.
func lineChargeMirrors(lines []*bill.Line) map[string]int {
	mirrors := make(map[string]int)
	for _, l := range lines {
		for _, ch := range l.Charges {
			if chargeIsExcise(ch.Key) {
				continue
			}
			mirrors[chargeMirrorKey(ch.Reason, ch.Amount, ch.Base)]++
		}
	}
	return mirrors
}

// lineDiscountMirrors is lineChargeMirrors' discount equivalent.
func lineDiscountMirrors(lines []*bill.Line) map[string]int {
	mirrors := make(map[string]int)
	for _, l := range lines {
		for _, d := range l.Discounts {
			mirrors[chargeMirrorKey(d.Reason, d.Amount, d.Base)]++
		}
	}
	return mirrors
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

// parseLineAllowanceCharge reads a line-level charge/discount's shared fields
// (OIOUBL decimal-factor percent, F-LIB228).
func parseLineAllowanceCharge(ac *AllowanceCharge, extKey cbc.Key) (parsedAllowanceCharge, error) {
	var out parsedAllowanceCharge
	amount, err := num.AmountFromString(ubl.NormalizeNumericString(ac.Amount.Value))
	if err != nil {
		return out, err
	}
	out.Amount = amount
	if ac.AllowanceChargeReasonCode != nil {
		out.Ext = tax.ExtensionsOf(cbc.CodeMap{
			extKey: cbc.Code(*ac.AllowanceChargeReasonCode),
		})
	}
	if ac.AllowanceChargeReason != nil {
		out.Reason = *ac.AllowanceChargeReason
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
	return out, nil
}

func goblLineCharge(ac *AllowanceCharge) (*bill.LineCharge, error) {
	p, err := parseLineAllowanceCharge(ac, untdid.ExtKeyCharge)
	if err != nil {
		return nil, err
	}
	return &bill.LineCharge{
		Reason:  p.Reason,
		Amount:  p.Amount,
		Base:    p.Base,
		Percent: p.Percent,
		Ext:     p.Ext,
	}, nil
}

func goblLineDiscount(ac *AllowanceCharge) (*bill.LineDiscount, error) {
	p, err := parseLineAllowanceCharge(ac, untdid.ExtKeyAllowance)
	if err != nil {
		return nil, err
	}
	return &bill.LineDiscount{
		Reason:  p.Reason,
		Amount:  p.Amount,
		Base:    p.Base,
		Percent: p.Percent,
		Ext:     p.Ext,
	}, nil
}
