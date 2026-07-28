package addon

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

// OIOUBLEndpointURI joins a scheme and code with a colon (e.g. "DK:CVR:12345674").
func OIOUBLEndpointURI(scheme, code cbc.Code) cbc.URI {
	return cbc.URI(scheme + ":" + code)
}

// normalizeParty migrates scheme/code inboxes to endpoints when none exist yet,
// and derives a DK:CVR endpoint from a Danish tax ID when still none is present.
func normalizeParty(p *org.Party) {
	if !partyHasEndpoint(p) {
		migrateInboxesToEndpoints(p)
	}
	if p.TaxID == nil || p.TaxID.Country != "DK" || p.TaxID.Code == cbc.CodeEmpty {
		return
	}
	if !partyHasEndpoint(p) {
		p.Endpoints = []*org.Endpoint{{
			URI: OIOUBLEndpointURI(SchemeDKCVR, p.TaxID.Code),
		}}
	}
	// Derive a CVR legal entity from the tax ID as a fallback
	if !hasLegalIdentity(p) {
		p.Identities = append(p.Identities, &org.Identity{
			Scope: org.IdentityScopeLegal,
			Code:  p.TaxID.Code,
		})
	}
}

// migrateInboxesToEndpoints converts each scheme/code org.Inbox into the
// equivalent org.Endpoint and drops it. Email/URL inboxes carry no scheme/
// code participant and are left untouched.
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

func partyHasEndpoint(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return true
	}
	for _, ep := range p.Endpoints {
		if ep != nil && ep.URI != "" {
			return true
		}
	}
	return false
}

func partyHasOIOUBLLegalID(val any) bool {
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
