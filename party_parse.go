package dkoioubl

import (
	"strings"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

// Reverses the wire-only DK prefix on DK:CVR/DK:SE identifiers (F-LIB180/184/190/196).
func (ui *Invoice) stripPartyFlavor() {
	stripParty(ui.AccountingSupplierParty.Party)
	stripParty(ui.AccountingCustomerParty.Party)
	stripParty(ui.PayeeParty)
	stripParty(ui.TaxRepresentativeParty)
}

func stripParty(p *ubl.Party) {
	if p == nil {
		return
	}
	if p.EndpointID != nil {
		p.EndpointID.Value = dkUnprefixed(&p.EndpointID.SchemeID, p.EndpointID.Value)
	}
	for i := range p.PartyIdentification {
		id := p.PartyIdentification[i].ID
		if id == nil {
			continue
		}
		id.Value = dkUnprefixed(id.SchemeID, id.Value)
	}
	for i := range p.PartyTaxScheme {
		pts := &p.PartyTaxScheme[i]
		if pts.CompanyID != nil {
			pts.CompanyID.Value = dkUnprefixed(pts.CompanyID.SchemeID, pts.CompanyID.Value)
		}
		stripTaxSchemeWire(pts.TaxScheme)
	}
	if le := p.PartyLegalEntity; le != nil && le.CompanyID != nil {
		le.CompanyID.Value = dkUnprefixed(le.CompanyID.SchemeID, le.CompanyID.Value)
	}
}

// Restores cac:Contact/cbc:ID onto the first contact person's identity
// (F-INV051); the generic parser reads Contact/Name but not Contact/ID.
func decoratePartyContact(p *org.Party, wire *ubl.Party) {
	if p == nil || wire == nil || wire.Contact == nil || wire.Contact.ID == nil || len(p.People) == 0 {
		return
	}
	code := cleanString(*wire.Contact.ID)
	if code == "" {
		return
	}
	p.People[0].Identities = []*org.Identity{{Code: cbc.Code(code)}}
}

// Promotes a DK:CVR PartyIdentification to a legal identity, since the
// generic parser only marks Scope=legal for PartyLegalEntity/CompanyID.
// DK:SE is excluded: F-LIB189 restricts PartyLegalEntity/CompanyID to
// DK:CVR/DK:CPR/ZZZ, DK:SE is only valid on PartyTaxScheme (F-LIB195).
func decoratePartyLegalIdentity(p *org.Party) {
	if p == nil || hasLegalIdentity(p) {
		return
	}
	for _, id := range p.Identities {
		if id != nil && id.Ext.Get(iso.ExtKeySchemeID).String() == schemeDKCVR {
			id.Scope = org.IdentityScopeLegal
			return
		}
	}
}

func hasLegalIdentity(p *org.Party) bool {
	for _, id := range p.Identities {
		if id != nil && id.Scope == org.IdentityScopeLegal {
			return true
		}
	}
	return false
}

// dkUnprefixed strips the wire-only "DK" prefix OIOUBL mandates on
// DK:CVR/DK:SE identifier values, leaving every other scheme's value as-is.
func dkUnprefixed(schemeID *string, value string) string {
	if schemeID == nil {
		return value
	}
	switch *schemeID {
	case schemeDKCVR, schemeDKSE:
		return strings.TrimPrefix(value, "DK")
	}
	return value
}
