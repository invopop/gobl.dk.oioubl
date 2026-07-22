package addon

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
)

// OIOUBLEndpointURI joins a scheme and code with a colon (e.g. "DK:CVR:12345674").
func OIOUBLEndpointURI(scheme, code cbc.Code) cbc.URI {
	return cbc.URI(scheme.String() + ":" + code.String())
}

// normalizeParty migrates a scheme/code inbox to an org.Endpoint and derives a
// DK:CVR endpoint from a Danish tax ID when none is present.
func normalizeParty(p *org.Party) {
	if len(p.Endpoints) == 0 {
		migrateInboxesToEndpoints(p)
	}
	if p.TaxID == nil || p.TaxID.Country != "DK" || p.TaxID.Code == cbc.CodeEmpty {
		return
	}
	if len(p.Endpoints) == 0 {
		p.Endpoints = append(p.Endpoints, &org.Endpoint{
			URI: OIOUBLEndpointURI(SchemeDKCVR, p.TaxID.Code),
		})
	}
	// OIOUBL's PartyLegalEntity/CompanyID is the CVR; set it explicitly as a
	// legal identity. Untouched if one already exists.
	if !hasLegalIdentity(p) {
		p.Identities = append(p.Identities, &org.Identity{
			Scope: org.IdentityScopeLegal,
			Code:  p.TaxID.Code,
		})
	}
}

// migrateInboxesToEndpoints converts each scheme/code org.Inbox into the
// equivalent org.Endpoint and drops it (org.Inbox is deprecated). Email/URL
// inboxes carry no scheme/code participant and are left untouched.
func migrateInboxesToEndpoints(p *org.Party) {
	kept := p.Inboxes[:0]
	for _, in := range p.Inboxes {
		if in == nil {
			continue
		}
		if in.Scheme == cbc.CodeEmpty || in.Code == cbc.CodeEmpty {
			kept = append(kept, in)
			continue
		}
		p.Endpoints = append(p.Endpoints, &org.Endpoint{
			Label: in.Label,
			URI:   OIOUBLEndpointURI(in.Scheme, in.Code),
		})
	}
	p.Inboxes = kept
}

// hasLegalIdentity reports whether the party already carries a legal-scope identity.
func hasLegalIdentity(p *org.Party) bool {
	for _, id := range p.Identities {
		if id != nil && id.Scope == org.IdentityScopeLegal {
			return true
		}
	}
	return false
}

// partyRoleRules returns the endpoint and legal-ID assertions shared by the
// supplier and customer fields; only the endpoint's schematron citation differs by role.
func partyRoleRules(role string, endpointNum, legalIDNum rules.Code, endpointCitation string) []rules.Def {
	return []rules.Def{
		rules.Assert(endpointNum, role+" must have an endpoint ("+endpointCitation+")",
			is.Func("has endpoint", partyHasEndpoint)),
		rules.Assert(legalIDNum, role+" requires a legal identity or a Danish tax ID for the OIOUBL PartyLegalEntity/CompanyID (F-LIB187)",
			is.Func("has an OIOUBL legal company ID", partyHasOIOUBLLegalID)),
	}
}

func partyHasEndpoint(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return true
	}
	return len(p.Endpoints) > 0
}

func partyHasOIOUBLLegalID(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return true
	}
	// A party with no name has no PartyLegalEntity, so F-LIB187 (its CompanyID) can't apply.
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

// firstPersonHasIdentityCode reports whether the first contact person carries an
// identity code, mapped to the OIOUBL cac:Contact/cbc:ID
func firstPersonHasIdentityCode(val any) bool {
	people, ok := val.([]*org.Person)
	if !ok || len(people) == 0 {
		return true
	}
	p := people[0]
	return p != nil && len(p.Identities) > 0 && p.Identities[0] != nil && !p.Identities[0].Code.IsEmpty()
}
