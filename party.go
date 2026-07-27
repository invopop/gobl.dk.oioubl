package oioubl

import (
	"strings"

	"github.com/invopop/gobl.dk.oioubl/addon"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

// OIOUBL symbolic schemes for company identifiers (F-LIB179/189/195); the CVR
// endpoint scheme is shared with the addon's own endpoint derivation.
const (
	schemeDKCVR = string(addon.SchemeDKCVR)
	schemeDKSE  = "DK:SE"
	schemeDKCPR = "DK:CPR"
	schemeZZZ   = "ZZZ"
)

// applyParties reworks the parties, their addresses and the delivery: first add
// what the base left out, then fix what it did build.
func (ui *Invoice) applyParties(inv *bill.Invoice) {
	duties := dutiesByCode(inv)
	// With an ordering seller the base already swapped these two around.
	supplier, taxRep := inv.Supplier, (*org.Party)(nil)
	if inv.Ordering != nil && inv.Ordering.Seller != nil {
		supplier, taxRep = inv.Ordering.Seller, inv.Supplier
	}
	addPartyDetails(ui.AccountingSupplierParty.Party, supplier)
	addPartyDetails(ui.AccountingCustomerParty.Party, inv.Customer)
	addPartyDetails(ui.TaxRepresentativeParty, taxRep)
	if inv.Payment != nil {
		addPayeeDetails(ui.PayeeParty, inv.Payment.Payee)
	}

	fixParty(ui.AccountingSupplierParty.Party, duties)
	fixParty(ui.AccountingCustomerParty.Party, duties)
	fixParty(ui.PayeeParty, duties)
	fixTaxRepParty(ui.TaxRepresentativeParty, duties)

	if len(ui.Delivery) > 0 {
		applyDelivery(ui.Delivery[0], inv.Delivery)
	}
}

// addPartyDetails copies over the party details the base's NewParty doesn't
// produce: the contact ID, the endpoint and the fuller address.
func addPartyDetails(p *ubl.Party, party *org.Party) {
	if p == nil || party == nil {
		return
	}
	applyPartyContact(p, party)
	applyPartyEndpoint(p, party)
	applyAddress(p.PostalAddress, firstAddress(party.Addresses))
}

// addPayeeDetails puts the payee's address and endpoint back. The shared base
// leaves the address out because EN 16931 forbids it; OIOUBL allows it.
func addPayeeDetails(p *ubl.Party, payee *org.Party) {
	if p == nil || payee == nil {
		return
	}
	if p.PostalAddress == nil {
		p.PostalAddress = newPostalAddress(payee.Addresses)
	}
	addPartyDetails(p, payee)
}

// applyPartyContact adds the mandatory cbc:ID (F-INV051), sourced from the
// first person's identity rather than fabricated.
func applyPartyContact(p *ubl.Party, party *org.Party) {
	if len(party.People) == 0 {
		return
	}
	ids := party.People[0].Identities
	if len(ids) == 0 || ids[0].Code == "" {
		return
	}
	code := ids[0].Code.String()
	if p.Contact == nil {
		p.Contact = &ubl.Contact{}
	}
	p.Contact.ID = &code
}

// applyPartyEndpoint overrides gobl.ubl's inbox-derived EndpointID with the
// OIOUBL endpoint URI (scheme+code emitted 1:1), when the party carries one.
func applyPartyEndpoint(p *ubl.Party, party *org.Party) {
	for _, ep := range party.Endpoints {
		if ep == nil {
			continue
		}
		if scheme, value, ok := splitEndpointURI(ep.URI.String()); ok {
			p.EndpointID = &ubl.EndpointID{SchemeID: scheme.String(), Value: value.String()}
			return
		}
	}
}

func splitEndpointURI(uri string) (scheme, code cbc.Code, ok bool) {
	i := strings.LastIndex(uri, ":")
	if i <= 0 || i == len(uri)-1 {
		return "", "", false
	}
	return cbc.Code(uri[:i]), cbc.Code(uri[i+1:]), true
}

// dkPrefixed adds the "DK" prefix OIOUBL mandates on DK:CVR/DK:SE values (F-LIB180/184), if absent.
func dkPrefixed(value string) string {
	if strings.HasPrefix(value, "DK") {
		return value
	}
	return "DK" + value
}

// applyCompanyID picks the scheme for a company ID: one the party already has
// wins, otherwise Danish parties get danishScheme and everyone else ZZZ.
func applyCompanyID(id *ubl.IDType, danishScheme string, danish bool) {
	if id == nil {
		return
	}
	if id.SchemeID != nil {
		switch *id.SchemeID {
		case schemeDKCVR, schemeDKSE:
			id.Value = dkPrefixed(id.Value)
			return
		case schemeDKCPR, schemeZZZ:
			return
		}
	}
	if danish {
		id.SchemeID = &danishScheme
		id.Value = dkPrefixed(id.Value)
		return
	}
	scheme := schemeZZZ
	id.SchemeID = &scheme
}

// fixParty corrects a finished party for OIOUBL: Danish numbers get their DK
// prefix, a missing name falls back to the registered one, and every company ID
// gets a scheme.
func fixParty(p *ubl.Party, duties map[string]exciseDuty) {
	if p == nil {
		return
	}
	if p.EndpointID != nil && p.EndpointID.SchemeID == schemeDKCVR {
		// OIOUBL CVR endpoints must carry the DK-prefixed form (F-LIB180).
		p.EndpointID.Value = dkPrefixed(p.EndpointID.Value)
	}
	if p.PartyName == nil && len(p.PartyIdentification) == 0 {
		if p.PartyLegalEntity != nil && p.PartyLegalEntity.RegistrationName != nil {
			p.PartyName = &ubl.PartyName{
				Name: *p.PartyLegalEntity.RegistrationName,
			}
		}
	}
	if p.PostalAddress != nil && p.PostalAddress.AddressFormatCode == nil {
		// gobl.ubl's NewParty sets only the tax ID's country here, with no AddressFormatCode; stamp the one OIOUBL requires.
		p.PostalAddress.AddressFormatCode = newAddressFormatCode(addressStructuredLax)
	}
	danish := partyIsDanish(p)
	for i := range p.PartyTaxScheme {
		pts := &p.PartyTaxScheme[i]
		applyCompanyID(pts.CompanyID, schemeDKSE, danish)
		applyPartyTaxScheme(pts.TaxScheme, duties)
	}
	if p.PartyLegalEntity != nil {
		if p.PartyLegalEntity.CompanyID == nil {
			// OIOUBL requires CompanyID whenever PartyLegalEntity is present (F-LIB187/189); drop the element rather than leave it incomplete.
			p.PartyLegalEntity = nil
		} else {
			applyCompanyID(p.PartyLegalEntity.CompanyID, schemeDKCVR, danish)
		}
	}
	applyPartyIdentifications(p)
}

// fixTaxRepParty strips a tax representative back to the name, tax scheme and
// address OIOUBL allows there (F-LIB357/358/361/362), then fixes it as usual.
func fixTaxRepParty(p *ubl.Party, duties map[string]exciseDuty) {
	if p == nil {
		return
	}
	p.EndpointID = nil
	p.PartyIdentification = nil
	p.PartyLegalEntity = nil
	p.Contact = nil
	fixParty(p, duties)
}

// applyPartyIdentifications DK-prefixes DK:CVR/DK:SE PartyIdentification values
// (F-LIB184); schemes are expected to be OIOUBL-symbolic already (F-LIB183).
func applyPartyIdentifications(p *ubl.Party) {
	for i := range p.PartyIdentification {
		id := p.PartyIdentification[i].ID
		if id == nil || id.SchemeID == nil {
			continue
		}
		if s := *id.SchemeID; s == schemeDKCVR || s == schemeDKSE {
			id.Value = dkPrefixed(id.Value)
		}
	}
}

// partyIsDanish decides DK:SE/DK:CVR vs the ZZZ "other" scheme. The postal
// address country (stamped from the tax identity) is the reliable marker.
func partyIsDanish(p *ubl.Party) bool {
	return p.PostalAddress != nil &&
		p.PostalAddress.Country != nil &&
		p.PostalAddress.Country.IdentificationCode == "DK"
}
