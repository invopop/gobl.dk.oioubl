package addon

import (
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/tax"
)

// OIOUBLEndpointURI joins a scheme and code with a colon (e.g. "DK:CVR:12345674").
func OIOUBLEndpointURI(scheme, code string) string {
	return scheme + ":" + code
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
			URI: cbc.URI(OIOUBLEndpointURI(SchemeDKCVR, p.TaxID.Code.String())),
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
// OIOUBL's bank-transfer code (F-LIB100).
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

// normalizeTaxCombo strips the EN 16931 UNTDID tax-category ext; the OIOUBL code
// derives from the GOBL VAT key (set by en16931, which runs first), so it's lossless.
func normalizeTaxCombo(c *tax.Combo) {
	c.Ext = c.Ext.Delete(untdid.ExtKeyTaxCategory)
}

// normalizeTaxNote strips the same UNTDID tax-category extension from a tax note;
// the note's key identifies the rate it applies to.
func normalizeTaxNote(n *tax.Note) {
	n.Ext = n.Ext.Delete(untdid.ExtKeyTaxCategory)
}

// taxCategoryMapsToOIOUBL reports whether a GOBL VAT key has an OIOUBL
// taxcategoryid-1.1 equivalent (standard/zero/exempt/reverse-charge).
func taxCategoryMapsToOIOUBL(key cbc.Key) bool {
	switch key {
	case tax.KeyStandard, tax.KeyZero, tax.KeyExempt, tax.KeyReverseCharge, "":
		return true
	}
	return false
}
