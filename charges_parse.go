package oioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/num"
)

// stripAllowanceMultiplier rewrites a discount or fee percentage the way the
// generic parser expects it. OIOUBL states it as a fraction, so amount equals
// base times factor (F-LIB228), while EN 16931 states the percentage itself:
// the same 25% is 0.25 on an OIOUBL wire and 25 on an EN 16931 one.
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
