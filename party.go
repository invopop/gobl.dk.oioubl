package dkoioubl

import (
	"strings"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

// decoratePartyExtras adds the OIOUBL fields gobl.ubl's generic party/address
// builders don't produce, and adjusts the ones OIOUBL disagrees with. The
// base's own NewParty (called as part of ubl.ConvertInvoice, including for
// the ordering.seller tax-representative swap) already builds everything
// else, so there's no need to rebuild parties from scratch here.
func decoratePartyExtras(p *Party, party *org.Party) {
	if p == nil || party == nil {
		return
	}
	decoratePartyContact(p, party)
	decoratePartyEndpoint(p, party)
	decoratePartyAddress(p, party)
}

// decoratePartyContact adds the mandatory cbc:ID (F-INV051), sourced from the
// first person's identity rather than fabricated.
func decoratePartyContact(p *Party, party *org.Party) {
	if len(party.People) == 0 {
		return
	}
	ids := party.People[0].Identities
	if len(ids) == 0 || ids[0].Code == "" {
		return
	}
	code := ids[0].Code.String()
	if p.Contact == nil {
		p.Contact = &Contact{}
	}
	p.Contact.ID = &code
}

// decoratePartyEndpoint overrides gobl.ubl's inbox-derived EndpointID with the
// OIOUBL endpoint URI (scheme+code emitted 1:1), when the party carries one.
func decoratePartyEndpoint(p *Party, party *org.Party) {
	for _, ep := range party.Endpoints {
		if ep == nil {
			continue
		}
		if scheme, value, ok := parseOIOUBLEndpoint(ep.URI.String()); ok {
			p.EndpointID = &EndpointID{SchemeID: scheme.String(), Value: value.String()}
			return
		}
	}
}

// decoratePartyAddress applies decorateAddress to a party's postal address,
// using the party's first GOBL address as the source.
func decoratePartyAddress(p *Party, party *org.Party) {
	if p.PostalAddress == nil {
		return
	}
	var a *org.Address
	if len(party.Addresses) > 0 {
		a = party.Addresses[0]
	}
	decorateAddress(p.PostalAddress, a)
}

// decorateAddress replaces gobl.ubl's combined StreetName (street+number in
// one string) with OIOUBL's separate StreetName/BuildingNumber, adds the
// Postbox field the base doesn't map, drops LocationCoordinate (forbidden by
// F-LIB212), and stamps the mandatory AddressFormatCode (F-LIB025).
func decorateAddress(addr *PostalAddress, a *org.Address) {
	if addr == nil {
		return
	}
	addr.LocationCoordinate = nil
	addr.AddressFormatCode = newAddressFormatCode(addressStructuredLax)
	if a == nil {
		return
	}
	if a.Street != "" {
		addr.StreetName = &a.Street
	}
	if a.Number != "" {
		addr.BuildingNumber = &a.Number
	}
	if a.PostOfficeBox != "" {
		addr.Postbox = &a.PostOfficeBox
	}
}

// newAddressFormatCode builds the cbc:AddressFormatCode required on every OIOUBL address (F-LIB025).
func newAddressFormatCode(value string) *IDType {
	listID := listAddressFormat
	listAgencyID := agencyID
	return &IDType{
		ListID:       &listID,
		ListAgencyID: &listAgencyID,
		Value:        value,
	}
}

// OIOUBL addressformatcode-1.1 values. Outbound always emits StructuredLax (no
// mandatory sub-fields); StructuredID is recognized on inbound parse.
const (
	addressStructuredLax = "StructuredLax"
	addressStructuredID  = "StructuredID"
)

// OIOUBL symbolic schemes for participant and company identifiers (F-LIB179/189/195).
const (
	schemeDKCVR = "DK:CVR"
	schemeDKSE  = "DK:SE"
	schemeZZZ   = "ZZZ"
)

func parseOIOUBLEndpoint(uri string) (scheme, code cbc.Code, ok bool) {
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

// applyCompanyID stamps a CompanyID's OIOUBL scheme: a Danish party gets the
// Danish scheme + DK-prefixed value (F-LIB190/196); a foreign party gets ZZZ
// with its value unchanged, since a forced DK scheme + prefix is wire-fatal.
func applyCompanyID(id *IDType, danishScheme string, danish bool) {
	if id == nil {
		return
	}
	if danish {
		id.SchemeID = &danishScheme
		id.Value = dkPrefixed(id.Value)
		return
	}
	scheme := schemeZZZ
	id.SchemeID = &scheme
}

// applyParty rewrites an assembled party into OIOUBL 2.1 form: DK-prefixed CVR
// endpoint (F-LIB179/180), fallback PartyName, and DK:SE/DK:CVR company-ID schemes.
func applyParty(p *Party) {
	if p == nil {
		return
	}
	if p.EndpointID != nil && p.EndpointID.SchemeID == schemeDKCVR {
		// OIOUBL CVR endpoints must carry the DK-prefixed form (F-LIB180).
		p.EndpointID.Value = dkPrefixed(p.EndpointID.Value)
	}
	if p.PartyName == nil && len(p.PartyIdentification) == 0 {
		if p.PartyLegalEntity != nil && p.PartyLegalEntity.RegistrationName != nil {
			p.PartyName = &PartyName{
				Name: *p.PartyLegalEntity.RegistrationName,
			}
		}
	}
	if p.PostalAddress != nil && p.PostalAddress.AddressFormatCode == nil {
		// A tax identity with no address yields a bare PostalAddress lacking a format code.
		p.PostalAddress.AddressFormatCode = newAddressFormatCode(addressStructuredLax)
	}
	danish := partyIsDanish(p)
	for i := range p.PartyTaxScheme {
		pts := &p.PartyTaxScheme[i]
		applyCompanyID(pts.CompanyID, schemeDKSE, danish)
		applyTaxScheme(pts.TaxScheme)
	}
	if p.PartyLegalEntity != nil {
		applyCompanyID(p.PartyLegalEntity.CompanyID, schemeDKCVR, danish)
	}
	applyPartyIdentifications(p)
}

// applyTaxRepParty drops the elements OIOUBL forbids on a cac:TaxRepresentativeParty
// (EndpointID, PartyIdentification, PartyLegalEntity, Contact) before the standard pass.
func applyTaxRepParty(p *Party) {
	if p == nil {
		return
	}
	p.EndpointID = nil
	p.PartyIdentification = nil
	p.PartyLegalEntity = nil
	p.Contact = nil
	applyParty(p)
}

// applyPartyIdentifications DK-prefixes DK:CVR/DK:SE PartyIdentification values
// (F-LIB184); schemes are expected to be OIOUBL-symbolic already (F-LIB183).
func applyPartyIdentifications(p *Party) {
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
func partyIsDanish(p *Party) bool {
	return p.PostalAddress != nil &&
		p.PostalAddress.Country != nil &&
		p.PostalAddress.Country.IdentificationCode == "DK"
}
