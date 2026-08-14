package addon

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

// OIOUBLEndpointURI joins a scheme and code with a colon (e.g. "DK:CVR:12345674").
func OIOUBLEndpointURI(scheme, code cbc.Code) cbc.URI {
	return cbc.URI(scheme + ":" + code)
}

// normalizeParty derives the endpoint and legal identity a Danish party may omit.
func normalizeParty(p *org.Party) {
	if p.FirstEndpoint() == nil {
		migrateInboxesToEndpoints(p)
	}

	// Only a Danish tax ID gives us anything to derive from.
	if p.TaxID == nil || p.TaxID.Country != "DK" || p.TaxID.Code == cbc.CodeEmpty {
		return
	}

	// An inbox may already have supplied one.
	if p.FirstEndpoint() == nil {
		p.Endpoints = []*org.Endpoint{{
			URI: OIOUBLEndpointURI(SchemeDKCVR, p.TaxID.Code),
		}}
	}

	// OIOUBL wants the CVR as a legal entity too.
	if !hasLegalIdentity(p) {
		p.Identities = append(p.Identities, &org.Identity{
			Scope: org.IdentityScopeLegal,
			Code:  p.TaxID.Code,
		})
	}
}

// migrateInboxesToEndpoints converts each scheme/code org.Inbox into an org.Endpoint.
func migrateInboxesToEndpoints(p *org.Party) {
	kept := p.Inboxes[:0]
	for _, in := range p.Inboxes {
		// A URL or email inbox has no scheme:code to build a URI from.
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
