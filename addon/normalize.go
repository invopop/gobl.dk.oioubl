package addon

import (
	"strings"

	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/tax"
)

// OIOUBLEndpointURI builds a NemHandel participant endpoint: the symbolic scheme
// and code joined by a colon (e.g. "DK:CVR:12345674"). Unlike Peppol's endpoint,
// this is not a resolvable URI — NemHandel identifies participants by DK:CVR,
// DK:SE or GLN. The code is colon-free, so the scheme (which may itself contain a
// colon, e.g. DK:CVR) is recovered on the last colon when reading.
func OIOUBLEndpointURI(scheme, code string) string {
	return scheme + ":" + code
}

// ParseOIOUBLEndpoint splits a participant endpoint into its scheme and code on
// the last colon, returning ok=false for a value without one.
func ParseOIOUBLEndpoint(uri string) (scheme, code cbc.Code, ok bool) {
	i := strings.LastIndex(uri, ":")
	if i <= 0 || i == len(uri)-1 {
		return "", "", false
	}
	return cbc.Code(uri[:i]), cbc.Code(uri[i+1:]), true
}

// normalizeParty resolves a party's NemHandel participant to an org.Endpoint
// (org.Inbox is deprecated): it migrates a scheme/code inbox to the equivalent
// endpoint, and for a Danish party lacking one derives the DK:CVR participant from
// the tax ID. An explicit participant supplied by the producer is preserved.
func normalizeParty(p *org.Party) {
	if len(p.Endpoints) == 0 {
		migrateInboxesToEndpoints(p)
	}
	if p.TaxID == nil || p.TaxID.Country != "DK" || p.TaxID.Code == cbc.CodeEmpty {
		return
	}
	if len(p.Endpoints) == 0 {
		p.Endpoints = append(p.Endpoints, &org.Endpoint{
			URI: cbc.URI(OIOUBLEndpointURI(SchemeDKCVR, p.TaxID.Code.String())),
		})
	}
	// OIOUBL's PartyLegalEntity/CompanyID is the CVR; set it explicitly so the
	// converter maps it rather than deriving one. Untouched if one already exists.
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
		scheme := in.Scheme.String()
		p.Endpoints = append(p.Endpoints, &org.Endpoint{
			Label: in.Label,
			URI:   cbc.URI(OIOUBLEndpointURI(scheme, in.Code.String())),
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

// normalizePayInstructions rewrites the EN 16931 credit-transfer means to
// OIOUBL's code so the serializer emits the correct cbc:PaymentMeansCode.
func normalizePayInstructions(instr *pay.Instructions) {
	instr.Ext = oioublPaymentMeans(instr.Ext)
}

// oioublPaymentMeans rewrites EN 16931 credit-transfer means 30 to OIOUBL's
// bank-transfer code 31 (F-LIB100). Other means pass through unchanged.
func oioublPaymentMeans(ext tax.Extensions) tax.Extensions {
	if ext.Get(untdid.ExtKeyPaymentMeans) == "30" {
		return ext.Set(untdid.ExtKeyPaymentMeans, "31")
	}
	return ext
}

// normalizeTaxCombo strips the redundant EN 16931 UNTDID tax-category extension;
// the serializer derives the OIOUBL taxcategoryid-1.1 code from the GOBL VAT key.
// en16931 (required, runs first) sets that key, so the removal is lossless.
func normalizeTaxCombo(c *tax.Combo) {
	c.Ext = c.Ext.Delete(untdid.ExtKeyTaxCategory)
}

// normalizeTaxNote strips the same UNTDID tax-category extension from a tax note;
// the note's key identifies the rate it applies to.
func normalizeTaxNote(n *tax.Note) {
	n.Ext = n.Ext.Delete(untdid.ExtKeyTaxCategory)
}

// taxCategoryMapsToOIOUBL reports whether a GOBL VAT key has an OIOUBL
// taxcategoryid-1.1 equivalent: standard/zero/exempt/reverse-charge do, while
// export, intra-community and outside-scope do not. Gates the addon's category
// rules.
func taxCategoryMapsToOIOUBL(key cbc.Key) bool {
	switch key {
	case tax.KeyStandard, tax.KeyZero, tax.KeyExempt, tax.KeyReverseCharge, "":
		return true
	}
	return false
}

