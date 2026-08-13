package oioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/num"
)

// stripAllowanceMultiplier rewrites an OIOUBL discount or fee factor as the
// percentage the generic parser expects: 0.25 (F-LIB228) becomes 25.
func stripAllowanceMultiplier(ac *ubl.AllowanceCharge) {
	if ac == nil || ac.MultiplierFactorNumeric == nil {
		return
	}
	factor, err := num.AmountFromString(normalizeNumericString(*ac.MultiplierFactorNumeric))
	if err != nil {
		return // leave it be; the generic parser reports the bad value
	}
	// GOBL stores a percentage as its own factor, so the wire value is already
	// the internal form and keeps the precision it was written with.
	percent := num.MakePercentage(factor.Value(), factor.Exp()).StringWithoutSymbol()
	ac.MultiplierFactorNumeric = &percent
}
