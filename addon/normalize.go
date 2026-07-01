package addon

import (
	"strings"

	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/tax"
)

// oioublEndpointScheme is the OIOUBL endpoint-identifier scheme URI
// (urn:oioubl:scheme:endpointid-1.1, the codelist's declared "Identification
// Scheme"), the OIOUBL counterpart to Peppol's iso6523-actorid-upis. Participants
// are carried as org.Endpoint URIs of the form
// "urn:oioubl:scheme:endpointid-1.1::<scheme>:<value>" (e.g. DK:CVR:12345674),
// where <scheme> is one of the nine symbolic codelist values (DK:CVR, DK:SE, GLN,
// "ISO 6523" for OVT, …).
const oioublEndpointScheme = "urn:oioubl:scheme:endpointid-1.1"

// OIOUBLEndpointURI builds the OIOUBL participant endpoint URI for a symbolic
// scheme and participant code. The value is colon-free (CVR/SE/GLN/IBAN/CPR/…),
// so the scheme — which may itself contain a colon (DK:CVR) — is recovered on the
// last colon when reading.
func OIOUBLEndpointURI(scheme, code string) string {
	return oioublEndpointScheme + "::" + scheme + ":" + code
}

// ParseOIOUBLEndpoint splits an OIOUBL endpoint URI into its symbolic scheme and
// participant code, returning ok=false for any other URI. The participant code is
// colon-free, so the scheme is recovered up to the last colon.
func ParseOIOUBLEndpoint(uri string) (scheme, code string, ok bool) {
	rest, found := strings.CutPrefix(uri, oioublEndpointScheme+"::")
	if !found {
		return "", "", false
	}
	i := strings.LastIndex(rest, ":")
	if i <= 0 || i == len(rest)-1 {
		return "", "", false
	}
	return rest[:i], rest[i+1:], true
}

// normalizeParty resolves a party's NemHandel participant to an org.Endpoint
// under the OIOUBL endpoint-identifier scheme — the going-forward routing field,
// since org.Inbox is deprecated. It (1) migrates a scheme/code inbox to the
// equivalent endpoint, and (2) for a Danish party still lacking one, derives the
// CVR participant (DK:CVR) from the tax identity. An explicit DK:SE, GLN or
// foreign participant supplied by the producer is preserved.
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
	// Legal identity: OIOUBL's PartyLegalEntity/CompanyID is the CVR. Set it
	// explicitly so the converter maps it rather than fabricating one from the tax
	// ID. Left untouched if a legal identity already exists.
	if !hasLegalIdentity(p) {
		p.Identities = append(p.Identities, &org.Identity{
			Scope: org.IdentityScopeLegal,
			Code:  p.TaxID.Code,
		})
	}
}

// migrateInboxesToEndpoints converts each scheme/code org.Inbox into the
// equivalent OIOUBL org.Endpoint and drops it, since org.Inbox is deprecated in
// favour of org.Endpoint. The inbox scheme is used as-is (it must already be an
// OIOUBL scheme); email/URL inboxes carry no scheme/code participant and are
// left untouched.
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
// OIOUBL's code (see oioublPaymentMeans) so the gobl.ubl serializer emits the
// correct cbc:PaymentMeansCode. The paymentchannelcode-1.1 value is a pure
// function of the means, so the converter derives it directly rather than from an
// extension.
func normalizePayInstructions(instr *pay.Instructions) {
	instr.Ext = oioublPaymentMeans(instr.Ext)
}

// oioublPaymentMeans rewrites the EN 16931 credit-transfer means (UNTDID 4461 code
// 30) to OIOUBL's bank-transfer code 31, which OIOUBL's PaymentMeansCode codelist
// requires in its place (F-LIB100). Other means pass through unchanged.
func oioublPaymentMeans(ext tax.Extensions) tax.Extensions {
	if ext.Get(untdid.ExtKeyPaymentMeans) == "30" {
		return ext.Set(untdid.ExtKeyPaymentMeans, "31")
	}
	return ext
}

// normalizeTaxCombo strips the EN 16931 UNTDID tax-category extension. The
// gobl.ubl serializer derives the OIOUBL taxcategoryid-1.1 code from the GOBL VAT
// key, never from this extension, so it is always redundant noise in an OIOUBL
// document. en16931 normalizes first (it is required), setting the key, so
// removing it is lossless.
func normalizeTaxCombo(c *tax.Combo) {
	c.Ext = c.Ext.Delete(untdid.ExtKeyTaxCategory)
}

// normalizeTaxNote strips the same UNTDID tax-category extension from a tax note;
// the note's key identifies the rate it applies to.
func normalizeTaxNote(n *tax.Note) {
	n.Ext = n.Ext.Delete(untdid.ExtKeyTaxCategory)
}

// taxCategoryMapsToOIOUBL reports whether a GOBL VAT key has an OIOUBL
// taxcategoryid-1.1 equivalent. The gobl.ubl serializer maps the key to the
// OIOUBL code directly (standard → StandardRated, zero/exempt → ZeroRated as
// OIOUBL 2.1 has no exempt category, reverse-charge → ReverseCharge); this gates
// the addon's own document-type and category rules. Export, intra-community and
// outside-scope have no OIOUBL category.
func taxCategoryMapsToOIOUBL(key cbc.Key) bool {
	switch key {
	case tax.KeyStandard, tax.KeyZero, tax.KeyExempt, tax.KeyReverseCharge, "":
		return true
	}
	return false
}

