package addon

import (
	"fmt"

	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// validPaymentMeansCodes are the UNTDID 4461 means accepted for OIOUBL (F-LIB100).
var validPaymentMeansCodes = []cbc.Code{
	"1", "10", "20", "31", "42", "48", "49", "50", "58", "59", "93", "97",
}

func payInstructionsRules() *rules.Set {
	return rules.For(new(pay.Instructions),
		rules.Field("ext",
			rules.AssertIfPresent("01", fmt.Sprintf("tax extension '%s' must use a payment means code OIOUBL supports (F-LIB100)", untdid.ExtKeyPaymentMeans),
				tax.ExtensionsHasCodes(untdid.ExtKeyPaymentMeans, validPaymentMeansCodes...)),
		),
		rules.When(is.Func("domestic bank transfer (means 42)", isDomesticTransfer),
			rules.Field("credit_transfer",
				rules.Each(rules.Field("clearing",
					rules.Assert("02", "a domestic transfer needs a four-digit bank clearing code (F-LIB124 / F-LIB130)",
						is.Present, is.Matches(`^\d{4}$`)),
				)),
			),
		),
	)
}

// isDomesticTransfer reports whether the payment means is a Danish domestic bank transfer (UNTDID 42).
func isDomesticTransfer(val any) bool {
	instr, ok := val.(*pay.Instructions)
	return ok && instr != nil && instr.Ext.Get(untdid.ExtKeyPaymentMeans) == "42"
}
