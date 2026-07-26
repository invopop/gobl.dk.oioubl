package oioubl

import (
	"strings"

	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

// stripParties reverses the wire-only DK prefix OIOUBL puts on Danish company
// numbers (F-LIB180/184/190/196).
func (ui *Invoice) stripParties() {
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

// addPartyContact keeps the contact's ID, which the generic parser drops --
// it reads Contact/Name but not Contact/ID (F-INV051).
func addPartyContact(p *org.Party, wire *ubl.Party) {
	if p == nil || wire == nil || wire.Contact == nil || wire.Contact.ID == nil || len(p.People) == 0 {
		return
	}
	code := cleanString(*wire.Contact.ID)
	if code == "" {
		return
	}
	p.People[0].Identities = []*org.Identity{{Code: cbc.Code(code)}}
}

// markLegalIdentity marks which identity is the official company number. Only
// CVR and CPR count: OIOUBL allows no other scheme there (F-LIB189).
func markLegalIdentity(p *org.Party) {
	if p == nil || hasLegalIdentity(p) {
		return
	}
	for _, id := range p.Identities {
		if id == nil {
			continue
		}
		switch id.Ext.Get(iso.ExtKeySchemeID).String() {
		case schemeDKCVR, schemeDKCPR:
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

// dkUnprefixed strips the wire-only "DK" prefix from a Danish company number,
// leaving every other scheme alone.
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
