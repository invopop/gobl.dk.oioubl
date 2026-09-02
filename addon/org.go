package addon

import (
	"strings"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

// OIOUBLEndpointURI joins a scheme and code with a colon (e.g. "DK:CVR:12345674").
func OIOUBLEndpointURI(scheme, code cbc.Code) cbc.URI {
	return cbc.URI(scheme + ":" + code)
}

// endpointSchemes lists the registers an OIOUBL EndpointID may name
// (F-LIB179, schematron 1.17.2; invoices and responses share the list).
var endpointSchemes = map[cbc.Code]bool{
	"GLN": true, "DUNS": true, "IBAN": true,
	"DK:P": true, "DK:CVR": true, "DK:CPR": true, "DK:SE": true, "DK:VANS": true,
	"FR:SIRET": true, "SE:ORGNR": true, "FI:OVT": true, "FI:ORGNR": true,
	"IT:FTI": true, "IT:SIA": true, "IT:SECETI": true, "IT:CF": true, "IT:IPA": true,
	"NO:ORGNR": true, "AT:GOV": true, "AT:CID": true, "AT:KUR": true, "IS:KT": true,
	"EU:REID": true,
	"AD:VAT":  true, "AL:VAT": true, "AT:VAT": true, "BA:VAT": true, "BE:VAT": true,
	"BG:VAT": true, "CH:VAT": true, "CY:VAT": true, "CZ:VAT": true, "DE:VAT": true,
	"EE:VAT": true, "ES:VAT": true, "EU:VAT": true, "FI:VAT": true, "GB:VAT": true,
	"GR:VAT": true, "HR:VAT": true, "HU:VAT": true, "IE:VAT": true, "IT:VAT": true,
	"LI:VAT": true, "LT:VAT": true, "LU:VAT": true, "LV:VAT": true, "MC:VAT": true,
	"ME:VAT": true, "MK:VAT": true, "MT:VAT": true, "NL:VAT": true, "NO:VAT": true,
	"PL:VAT": true, "PT:VAT": true, "RO:VAT": true, "RS:VAT": true, "SE:VAT": true,
	"SI:VAT": true, "SK:VAT": true, "SM:VAT": true, "TR:VAT": true, "VA:VAT": true,
}

// OIOUBLEndpoint returns the party's first endpoint whose URI ("scheme:code")
// names a register OIOUBL accepts (F-LIB179), or nil when it has none. A party
// may also carry endpoints for other networks (e.g. Peppol's
// iso6523-actorid-upis, added by the en16931 addon); those are not errors,
// they are just not usable here.
func OIOUBLEndpoint(p *org.Party) *org.Endpoint {
	if p == nil {
		return nil
	}
	return oioublEndpoint(p.Endpoints)
}

func oioublEndpoint(eps []*org.Endpoint) *org.Endpoint {
	for _, ep := range eps {
		if ep == nil {
			continue
		}
		uri := ep.URI.String()
		if i := strings.LastIndex(uri, ":"); i > 0 && endpointSchemes[cbc.Code(uri[:i])] {
			return ep
		}
	}
	return nil
}

// partyHasOIOUBLEndpoint reports whether at least one endpoint names a
// register OIOUBL accepts. An empty list passes; presence has its own rules.
func partyHasOIOUBLEndpoint(val any) bool {
	eps, ok := val.([]*org.Endpoint)
	if !ok || len(eps) == 0 {
		return true
	}
	return oioublEndpoint(eps) != nil
}

// normalizeParty derives the endpoint and legal identity a Danish party may
// omit. An endpoint for another network (e.g. Peppol) does not count as
// having one: OIOUBL needs its own.
func normalizeParty(p *org.Party) {
	if OIOUBLEndpoint(p) == nil {
		migrateInboxesToEndpoints(p)
	}

	// Only a Danish tax ID gives us anything to derive from.
	if p.TaxID == nil || p.TaxID.Country != "DK" || p.TaxID.Code == cbc.CodeEmpty {
		return
	}

	// An inbox may already have supplied one; endpoints for other networks
	// are kept alongside the derived one.
	if OIOUBLEndpoint(p) == nil {
		p.Endpoints = append(p.Endpoints, &org.Endpoint{
			URI: OIOUBLEndpointURI(SchemeDKCVR, p.TaxID.Code),
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
