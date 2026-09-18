package addon

import (
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

// normalizeParty derives the endpoint and legal identity a Danish party may
// omit. An endpoint on another network does not count as having one: OIOUBL
// needs one naming a register it accepts.
//
// EN 16931 gives a party a single electronic address (BT-34, BT-49). A party
// already addressed by a participant identifier, even one naming a register
// OIOUBL cannot route to, gets no second one, whether from an inbox or from
// its tax ID: deriving it would be refused, and F-LIB179 already says what is
// wrong. Endpoints on other networks are kept alongside.
func normalizeParty(p *org.Party) {
	if OIOUBLEndpoint(p) == nil && !hasParticipantEndpoint(p) {
		migrateInboxesToEndpoints(p)
	}
	normalizeEndpoints(p)

	// Only a Danish tax ID gives us anything to derive from.
	if p.TaxID == nil || p.TaxID.Country != "DK" || p.TaxID.Code == cbc.CodeEmpty {
		return
	}

	// An inbox or an existing participant identifier may already have supplied
	// one.
	if OIOUBLEndpoint(p) == nil && !hasParticipantEndpoint(p) {
		p.Endpoints = append(p.Endpoints, &org.Endpoint{
			URI: OIOUBLEndpointURI(RegisterDKCVR, p.TaxID.Code),
		})
	}

	// OIOUBL wants the CVR as a legal entity too.
	if !hasLegalIdentity(p) {
		p.Identities = append(p.Identities, &org.Identity{
			Scope: org.IdentityScopeLegal,
			Code:  p.TaxID.Code,
		})
	}
}

// normalizeEndpoints rewrites each endpoint OIOUBL can route onto the
// participant identifier its register is named by, and settles the code on
// what that identifier expects.
func normalizeEndpoints(p *org.Party) {
	for _, ep := range p.Endpoints {
		if ep == nil {
			continue
		}
		name, code, ok := SplitEndpointURI(ep.URI)
		if !ok {
			continue
		}
		if uri := OIOUBLEndpointURI(name, code); uri != "" {
			// An empty URI means nothing addressable was left, such as a code
			// that was only the prefix; leave it for validation to refuse.
			ep.URI = uri
		}
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
		uri := OIOUBLEndpointURI(in.Scheme, in.Code)
		if uri == "" {
			// Nothing addressable, such as a code that was only the "DK"
			// prefix; keep the inbox rather than mint an empty endpoint.
			kept = append(kept, in)
			continue
		}
		p.Endpoints = append(p.Endpoints, &org.Endpoint{
			Label: in.Label,
			URI:   uri,
		})
	}
	p.Inboxes = kept
}

// hasParticipantEndpoint reports whether the party is already addressed by a
// participant identifier, whatever register it names.
func hasParticipantEndpoint(p *org.Party) bool {
	for _, ep := range p.Endpoints {
		if ep != nil && ep.URI.Scheme() == iso.ActorIDScheme {
			return true
		}
	}
	return false
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
