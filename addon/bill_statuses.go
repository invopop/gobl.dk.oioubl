package addon

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
)

// billStatusRules returns the OIOUBL 2.1 rule set for bill.Status,
// targeting Invoice Response (UBL ApplicationResponse with Type "response").
func billStatusRules() *rules.Set {
	return rules.For(new(bill.Status),
		rules.When(isResponseType,
			rules.Field("code",
				rules.Assert("05", "code is required (F-APR005)", is.Present),
			),
			// The converter sends the response FROM the responder TO the
			// originator: the customer becomes the SenderParty (F-APR008
			// endpoint, F-APR040 one PartyLegalEntity) and the supplier becomes
			// the ReceiverParty (F-APR012 endpoint, F-APR041 at most one
			// PartyLegalEntity). The citations below follow that mapping rather
			// than the GOBL field name.
			rules.Field("supplier",
				rules.Assert("01", "supplier must have an endpoint or inbox (F-APR012)",
					is.Func("has endpoint or inbox", partyHasEndpointOrInbox)),
				rules.Assert("06", "supplier must have a tax ID or identities (F-APR041)",
					is.Func("has tax id or identities", partyHasTaxIDOrIdentities)),
				rules.Assert("07", "supplier must have a name or identification (F-LIB022)",
					is.Func("has name or identification", partyHasNameOrIdentification)),
			),
			rules.Field("customer",
				rules.Assert("02", "customer is required for a response", is.Present),
				rules.Assert("03", "customer must have an endpoint or inbox (F-APR008)",
					is.Func("has endpoint or inbox", partyHasEndpointOrInbox)),
				rules.Assert("08", "customer must have a name or identification (F-LIB022)",
					is.Func("has name or identification", partyHasNameOrIdentification)),
			),
			rules.Field("lines",
				// An OIOUBL ApplicationResponse carries exactly one Response for one
				// referenced document (F-APR051 / F-APR054); the converter maps each
				// status line to a DocumentResponse, so it must hold a single line.
				rules.Assert("16", "a response carries exactly one document response (F-APR051 / F-APR054)", is.Length(1, 1)),
				rules.Each(
					// Only the four events the responsecode-1.1 code list accepts are
					// representable; everything else (issued, processing, paid, …) has
					// no OIOUBL response code (F-APR018).
					rules.Field("key",
						rules.Assert("15", "response status event must be one OIOUBL supports (F-APR018)",
							is.In(
								bill.StatusLineAccepted,
								bill.StatusLineRejected,
								bill.StatusLineAcknowledged,
								bill.StatusLineError,
							)),
					),
					rules.Field("doc",
						rules.Assert("04", "line document reference is required for a response (cf. F-APR016, F-APR025)", is.Present),
					),
				),
			),
		),
	)
}

var isResponseType = is.Func("response status type", func(val any) bool {
	st, ok := val.(*bill.Status)
	return ok && st != nil && st.Type == bill.StatusTypeResponse
})

// partyHasEndpointOrInbox accepts either routing model: org.Endpoint is the
// going-forward form for the wire cbc:EndpointID, while org.Inbox documents
// produced before the endpoint migration remain valid.
func partyHasEndpointOrInbox(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return true
	}
	return len(p.Endpoints) > 0 || len(p.Inboxes) > 0
}

func partyHasTaxIDOrIdentities(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return true
	}
	return p.TaxID != nil || len(p.Identities) > 0
}

func partyHasNameOrIdentification(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return true
	}
	if p.Name != "" {
		return true
	}
	legalSeen := false
	for _, id := range p.Identities {
		if id == nil || id.Scope == org.IdentityScopeTax {
			continue
		}
		if id.Scope == org.IdentityScopeLegal {
			// The first legal-scope identity becomes the CompanyID; a further one
			// falls through to a PartyIdentification.
			if legalSeen {
				return true
			}
			legalSeen = true
			continue
		}
		return true
	}
	return false
}
