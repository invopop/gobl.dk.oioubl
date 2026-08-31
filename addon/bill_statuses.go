package addon

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
)

// Rule citations reference the OIOUBL ApplicationResponse schematron (F-APR).

// billStatusRules returns the OIOUBL 2.1 rule set for bill.Status,
// targeting Invoice Response (UBL ApplicationResponse with Type "response").
func billStatusRules() *rules.Set {
	return rules.For(new(bill.Status),
		rules.When(isResponseType,
			rules.Field("code",
				rules.Assert("01", "code is required (F-APR005)", is.Present),
			),
			// Roles invert in a response: customer is the sender, supplier the
			// receiver, so the F-APR citations below follow that inversion.
			rules.Field("supplier",
				rules.Field("endpoints",
					rules.Assert("02", "supplier endpoint is required (F-APR012)", is.Present),
					rules.Assert("13", "at least one supplier endpoint must name a register OIOUBL accepts (F-LIB179)",
						is.Func("has an OIOUBL endpoint", partyHasOIOUBLEndpoint)),
				),
				rules.Assert("03", "supplier must have a tax ID code or an identity (F-APR041)",
					is.Func("has tax id code or identity", partyHasTaxIDOrIdentities)),
				rules.Assert("04", "supplier must have a name or identification (F-LIB022)",
					is.Func("has name or identification", partyHasNameOrIdentification)),
			),
			rules.Field("customer",
				rules.Assert("05", "customer is required for a response", is.Present),
				rules.Field("endpoints",
					rules.Assert("06", "customer endpoint is required (F-APR008)", is.Present),
					rules.Assert("14", "at least one customer endpoint must name a register OIOUBL accepts (F-LIB179)",
						is.Func("has an OIOUBL endpoint", partyHasOIOUBLEndpoint)),
				),
				rules.Assert("07", "customer must have a name or identification (F-LIB022)",
					is.Func("has name or identification", partyHasNameOrIdentification)),
				// The customer sends the response, and the SenderParty requires
				// exactly one PartyLegalEntity; a Danish tax ID satisfies this via
				// the identity derived in normalization.
				rules.Assert("12", "customer requires a legal identity for the OIOUBL PartyLegalEntity (F-APR040)",
					is.Func("has a legal identity", partyHasLegalIdentity)),
			),
			rules.Field("lines",
				// One Response per referenced document (F-APR051 / F-APR054).
				rules.Assert("08", "a response carries exactly one document response (F-APR051 / F-APR054)", is.Length(1, 1)),
				rules.Each(
					// Only these three keys are representable (F-APR018); the error
					// key is refused because every negative responsecode-1.1 answer
					// is a definitive rejection, not a recoverable error.
					rules.Field("key",
						rules.Assert("09", "response status event must be one OIOUBL supports (F-APR018)",
							is.In(
								bill.StatusLineAccepted,
								bill.StatusLineRejected,
								bill.StatusLineAcknowledged,
							)),
					),
					rules.Field("doc",
						rules.Assert("10", "line document reference is required for a response (cf. F-APR016, F-APR025)", is.Present),
					),
					// The key carries the meaning and the extension the exact wire
					// value, so a pair that contradicts itself is a data error.
					rules.Assert("11", "response code extension must agree with the status key",
						is.Func("response code matches the key", lineResponseCodeMatchesKey)),
				),
			),
		),
	)
}

// statusKeyForResponseCode names the status key each responsecode-1.1 value
// means: the business answers keep their meaning, the profile and technical
// ones collapse to acknowledged/rejected. All three rejects are terminal —
// OIOUBL expects a new document after any of them — so none maps to the
// error key, which the platform reads as recoverable.
var statusKeyForResponseCode = map[cbc.Code]cbc.Key{
	ResponseCodeBusinessAccept:  bill.StatusLineAccepted,
	ResponseCodeBusinessReject:  bill.StatusLineRejected,
	ResponseCodeProfileAccept:   bill.StatusLineAcknowledged,
	ResponseCodeTechnicalAccept: bill.StatusLineAcknowledged,
	ResponseCodeProfileReject:   bill.StatusLineRejected,
	ResponseCodeTechnicalReject: bill.StatusLineRejected,
}

// StatusKeyForResponseCode returns the bill.StatusLine key a responsecode-1.1
// value maps to, or empty when OIOUBL defines no such code.
func StatusKeyForResponseCode(code cbc.Code) cbc.Key {
	return statusKeyForResponseCode[code]
}

func lineResponseCodeMatchesKey(val any) bool {
	line, ok := val.(*bill.StatusLine)
	if !ok || line == nil {
		return true
	}
	code := line.Ext.Get(ExtKeyResponseCode)
	if code == "" {
		return true
	}
	return statusKeyForResponseCode[code] == line.Key
}

var isResponseType = is.Func("response status type", func(val any) bool {
	st, ok := val.(*bill.Status)
	return ok && st != nil && st.Type == bill.StatusTypeResponse
})

func partyHasTaxIDOrIdentities(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return true
	}
	// A codeless tax ID yields no CompanyID in the XML, so it does not count.
	if p.TaxID != nil && p.TaxID.Code != "" {
		return true
	}
	for _, id := range p.Identities {
		if id != nil && id.Code != "" {
			return true
		}
	}
	return false
}

func partyHasLegalIdentity(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return true
	}
	for _, id := range p.Identities {
		if id != nil && id.Scope == org.IdentityScopeLegal && !id.Code.IsEmpty() {
			return true
		}
	}
	return false
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
