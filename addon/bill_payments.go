package addon

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// billPaymentRules returns the OIOUBL 2.1 rule set for bill.Payment, targeting
// the Reminder (Rykker) mapped from the "request" payment type.
func billPaymentRules() *rules.Set {
	return rules.For(new(bill.Payment),
		rules.When(bill.PaymentTypeIn(bill.PaymentTypeRequest),
			rules.Field("code",
				rules.Assert("01", "payment code is required for the OIOUBL Reminder ID (F-REM010)", is.Present),
			),
			rules.Field("ext",
				rules.Assert("02", "an OIOUBL reminder sequence number is required (F-REM007)",
					tax.ExtensionsRequire(ExtKeyReminderSequence)),
			),
			rules.Field("supplier",
				rules.Field("endpoints",
					rules.Assert("03", "supplier endpoint is required (F-REM018)", is.Present),
				),
				rules.Assert("04", "supplier requires a legal identity or a Danish tax ID for the OIOUBL PartyLegalEntity (F-REM021 / F-LIB187)",
					is.Func("has an OIOUBL legal company ID", partyHasOIOUBLLegalID)),
			),
			rules.Field("customer",
				rules.Assert("05", "customer is required (F-REM024)", is.Present),
				rules.Field("endpoints",
					rules.Assert("06", "customer endpoint is required (F-REM025)", is.Present),
				),
				rules.Assert("07", "customer requires a legal identity or a Danish tax ID for the OIOUBL PartyLegalEntity (F-LIB187)",
					is.Func("has an OIOUBL legal company ID", partyHasOIOUBLLegalID)),
				rules.Field("people",
					rules.Assert("08", "a customer contact person is required for the OIOUBL Contact (F-REM071)", is.Present),
				),
			),
			rules.Field("payee",
				rules.Field("endpoints",
					rules.Assert("09", "payee endpoint is required (F-REM034)", is.Present),
				),
			),
		),
	)
}

// partyHasOIOUBLLegalID reports whether a named party can produce a non-empty
// OIOUBL PartyLegalEntity/CompanyID from a legal-scope identity (F-LIB187); a
// Danish tax ID satisfies it via the identity derived in normalization.
func partyHasOIOUBLLegalID(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return true
	}
	// A party with no name has no PartyLegalEntity, so its CompanyID can't apply.
	if p.Name == "" {
		return true
	}
	for _, id := range p.Identities {
		if id != nil && id.Scope == org.IdentityScopeLegal && !id.Code.IsEmpty() {
			return true
		}
	}
	return false
}
