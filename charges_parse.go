package oioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/num"
)

// stripAllowanceMultiplier restates a discount or fee percentage the way the
// generic parser reads it: 25% arrives as the fraction 0.25 (F-LIB228) and has
// to become 25.
func stripAllowanceMultiplier(ac *ubl.AllowanceCharge) {
	if ac == nil || ac.MultiplierFactorNumeric == nil {
		return
	}
	factor, err := num.AmountFromString(normalizeNumericString(*ac.MultiplierFactorNumeric))
	if err != nil {
		return // leave it be; the generic parser reports the bad value
	}
	// Multiplying by 100 moves the decimal point two places, so the value keeps
	// the precision it was written with: 0.25 is 25%, not 25.00%.
	exp := uint32(0)
	if factor.Exp() > 2 {
		exp = factor.Exp() - 2
	}
	percent := factor.Multiply(num.MakeAmount(100, 0)).Rescale(exp).String()
	ac.MultiplierFactorNumeric = &percent
}
