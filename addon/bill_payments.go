package addon

import (
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
)

// billPayTermsRules relaxes EN 16931 BR-CO-25: OIOUBL allows bare payment terms
// (ID + amount only), so the due-dates-or-notes requirement doesn't apply.
func billPayTermsRules() *rules.Set {
	return rules.For(new(pay.Terms),
		rules.Ignore("GOBL-EU-EN16931-PAY-TERMS-01"),
	)
}
