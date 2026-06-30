package addon

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// billPayTermsRules relaxes the EN 16931 BR-CO-25 payment-terms shape for OIOUBL,
// which allows bare terms (its official samples carry terms with only an ID and
// amount), so the due-dates-or-notes requirement does not apply.
func billPayTermsRules() *rules.Set {
	return rules.For(new(pay.Terms),
		rules.Ignore("GOBL-EU-EN16931-PAY-TERMS-01"),
	)
}

// billPaymentRules returns the OIOUBL 2.1 rule set for bill.Payment. The rules
// fire only for the "request" payment type, which is the GOBL document mapped to
// an OIOUBL Reminder (Rykker/dunning). Other payment types (receipt, advice) have
// no OIOUBL document and are left to GOBL core.
//
// GOBL core (paymentRules) already requires the type, issue date, currency
// (F-REM008), supplier (F-REM017), lines and at least one method; EN 16931
// registers no rules on bill.Payment at all. These rules close the OIOUBL gaps
// the converter cannot satisfy from absent data — there is no en16931
// over-enforcement on the request payment, so no rules.Ignore is required.
//
// Rules left to other layers: the Reminder totals math (F-REM079-086) is produced
// by core Calculate; the excluded-element, single-currency and cardinality
// assertions (e.g. F-REM002/003/065-068, F-REM069-072) are controlled by the
// gobl.ubl serializer, which never emits the offending shapes; the penalty-fee
// AllowanceCharge (F-REM094/097) has no bill.Payment field to map from.
func billPaymentRules() *rules.Set {
	return rules.For(new(bill.Payment),
		rules.When(bill.PaymentTypeIn(bill.PaymentTypeRequest),
			rules.Field("code",
				rules.Assert("01", "payment code is required for the OIOUBL Reminder ID (F-REM010)", is.Present),
			),
			rules.Field("ext",
				rules.Assert("02", "an OIOUBL reminder type is required (F-REM006 / F-REM061)",
					tax.ExtensionsRequire(ExtKeyReminderType)),
				rules.Assert("03", "an OIOUBL reminder sequence number is required (F-REM007)",
					tax.ExtensionsRequire(ExtKeyReminderSequence)),
			),
			rules.Field("supplier",
				rules.Assert("04", "supplier must have an ISO 6523 endpoint or inbox (F-REM018)",
					is.Func("has endpoint or inbox", partyHasEndpointOrInbox)),
				rules.Assert("05", "supplier requires a legal identity or a Danish tax ID for the OIOUBL PartyLegalEntity (F-REM021 / F-LIB187)",
					is.Func("has an OIOUBL legal company ID", partyHasOIOUBLLegalID)),
			),
			rules.Field("customer",
				rules.Assert("06", "customer is required (F-REM024)", is.Present),
				rules.Assert("07", "customer must have an ISO 6523 endpoint or inbox (F-REM025)",
					is.Func("has endpoint or inbox", partyHasEndpointOrInbox)),
				// The Reminder schematron does not mandate a customer PartyLegalEntity
				// (F-REM093 only caps it at one), but the gobl.ubl converter emits one
				// for any named party, and OIOUBL then requires its CompanyID (F-LIB187).
				rules.Assert("08", "customer requires a legal identity or a Danish tax ID for the OIOUBL PartyLegalEntity (F-LIB187)",
					is.Func("has an OIOUBL legal company ID", partyHasOIOUBLLegalID)),
				rules.Field("people",
					rules.Assert("09", "a customer contact person is required for the OIOUBL Contact (F-REM071)", is.Present),
				),
			),
			rules.Field("payee",
				rules.AssertIfPresent("10", "payee must have an ISO 6523 endpoint or inbox (F-REM034)",
					is.Func("has endpoint or inbox", partyHasEndpointOrInbox)),
			),
			rules.Field("methods",
				rules.Each(
					rules.When(is.Func("fik kortart 73 carrying a payment reference", fik73RecordWithReference),
						rules.Assert("11", "FIK payment id 73 must not carry a payment reference, OIOUBL has no element for it (F-LIB275)", is.Func("never", neverTrue)),
					),
				),
			),
		),
	)
}
