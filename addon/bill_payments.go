package addon

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// billPayTermsRules relaxes EN 16931 BR-CO-25: OIOUBL allows bare payment terms
// (ID + amount only), so the due-dates-or-notes requirement doesn't apply.
func billPayTermsRules() *rules.Set {
	return rules.For(new(pay.Terms),
		rules.Ignore("GOBL-EU-EN16931-PAY-TERMS-01"),
	)
}

// billPaymentRules returns the OIOUBL Reminder (Rykker) rules, applied only to the
// "request" payment type. en16931 has no bill.Payment rules, so nothing to Ignore.
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
				rules.Assert("03", "supplier must have an endpoint (F-REM018)",
					is.Func("has endpoint", partyHasEndpoint)),
				rules.Assert("04", "supplier requires a legal identity or a Danish tax ID for the OIOUBL PartyLegalEntity (F-REM021 / F-LIB187)",
					is.Func("has an OIOUBL legal company ID", partyHasOIOUBLLegalID)),
			),
			rules.Field("customer",
				rules.Assert("05", "customer is required (F-REM024)", is.Present),
				rules.Assert("06", "customer must have an endpoint (F-REM025)",
					is.Func("has endpoint", partyHasEndpoint)),
				// A named customer has a PartyLegalEntity, so OIOUBL requires its CompanyID (F-LIB187).
				rules.Assert("07", "customer requires a legal identity or a Danish tax ID for the OIOUBL PartyLegalEntity (F-LIB187)",
					is.Func("has an OIOUBL legal company ID", partyHasOIOUBLLegalID)),
				rules.Field("people",
					rules.Assert("08", "a customer contact person is required for the OIOUBL Contact (F-REM071)", is.Present),
				),
			),
			rules.Field("payee",
				rules.AssertIfPresent("09", "payee must have an endpoint (F-REM034)",
					is.Func("has endpoint", partyHasEndpoint)),
			),
		),
	)
}
