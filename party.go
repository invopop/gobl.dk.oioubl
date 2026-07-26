package dkoioubl

import (
	"strings"

	oioubl "github.com/invopop/gobl.dk.oioubl/addon"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

// applyPartyAndAddressFlavor adjusts the base's parties, addresses, and delivery for OIOUBL.
func (ui *Invoice) applyPartyAndAddressFlavor(inv *bill.Invoice) {
	supplierSrc := inv.Supplier
	if inv.Ordering != nil && inv.Ordering.Seller != nil {
		// addOrdering already swapped AccountingSupplierParty/TaxRepresentativeParty.
		supplierSrc = inv.Ordering.Seller
		applyPartyExtras(ui.TaxRepresentativeParty, inv.Supplier)
	}
	applyPartyExtras(ui.AccountingSupplierParty.Party, supplierSrc)
	applyPartyExtras(ui.AccountingCustomerParty.Party, inv.Customer)
	if inv.Payment != nil {
		applyPayeeExtras(ui.PayeeParty, inv.Payment.Payee)
	}

	applyParty(ui.AccountingSupplierParty.Party)
	applyParty(ui.AccountingCustomerParty.Party)
	applyParty(ui.PayeeParty)
	applyTaxRepParty(ui.TaxRepresentativeParty)

	if len(ui.Delivery) > 0 {
		applyDelivery(ui.Delivery[0], inv.Delivery)
	}
}

// applyPartyExtras adds the OIOUBL fields the base's NewParty doesn't
// produce, and adjusts the ones OIOUBL disagrees with.
func applyPartyExtras(p *ubl.Party, party *org.Party) {
	if p == nil || party == nil {
		return
	}
	applyPartyContact(p, party)
	applyPartyEndpoint(p, party)
	applyPartyAddress(p, party)
}

// applyPayeeExtras puts the payee's address and endpoint back. The shared
// base leaves the address out because EN 16931 forbids it; OIOUBL allows it.
func applyPayeeExtras(p *ubl.Party, payee *org.Party) {
	if p == nil || payee == nil {
		return
	}
	if p.PostalAddress == nil && len(payee.Addresses) > 0 {
		p.PostalAddress = newPostalAddress(payee.Addresses[0])
	}
	applyPartyExtras(p, payee)
}

// newPostalAddress fills the address fields applyAddress leaves alone.
func newPostalAddress(a *org.Address) *ubl.PostalAddress {
	if a == nil {
		return nil
	}
	addr := new(ubl.PostalAddress)
	if a.StreetExtra != "" {
		l := a.LineTwo()
		addr.AdditionalStreetName = &l
	}
	if a.Locality != "" {
		addr.CityName = &a.Locality
	}
	if a.Region != "" {
		addr.CountrySubentity = &a.Region
	}
	if a.Code != cbc.CodeEmpty {
		code := a.Code.String()
		addr.PostalZone = &code
	}
	if a.Country != "" {
		addr.Country = &ubl.Country{IdentificationCode: string(a.Country)}
	}
	return addr
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

// applyPartyAddress applies applyAddress to a party's postal address,
// using the party's first GOBL address as the source.
func applyPartyAddress(p *ubl.Party, party *org.Party) {
	if p.PostalAddress == nil {
		return
	}
	var a *org.Address
	if len(party.Addresses) > 0 {
		a = party.Addresses[0]
	}
	applyAddress(p.PostalAddress, a)
}

// applyAddress drops the forbidden LocationCoordinate (F-LIB212) and
// stamps the mandatory AddressFormatCode (F-LIB025), among other OIOUBL fields.
func applyAddress(addr *ubl.PostalAddress, a *org.Address) {
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
func newAddressFormatCode(value string) *ubl.IDType {
	listID := codelistAddressFormat
	listAgencyID := agencyID
	return &ubl.IDType{
		ListID:       &listID,
		ListAgencyID: &listAgencyID,
		Value:        value,
	}
}

// addressStructuredLax is the OIOUBL addressformatcode-1.1 value emitted on
// every outbound address (no mandatory sub-fields).
const addressStructuredLax = "StructuredLax"

// OIOUBL symbolic schemes for company identifiers (F-LIB179/189/195); the CVR
// endpoint scheme is shared with the addon's own endpoint derivation.
const (
	schemeDKCVR = string(oioubl.SchemeDKCVR)
	schemeDKSE  = "DK:SE"
	schemeDKCPR = "DK:CPR"
	schemeZZZ   = "ZZZ"
)

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

// applyCompanyID stamps a CompanyID's OIOUBL scheme. A symbolic OIOUBL scheme
// the base already carried from the identity's iso-scheme-id extension wins —
// a CPR-identified person must not be relabelled as a CVR company (F-LIB190/191)
// — with the wire-only DK prefix added on DK:CVR/DK:SE values (F-LIB190/196).
// Otherwise the party's country decides: Danish scheme + DK-prefixed value, or
// ZZZ + unchanged value.
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

// applyParty rewrites an assembled party into OIOUBL 2.1 form: DK-prefixed CVR
// endpoint (F-LIB179/180), fallback PartyName, and DK:SE/DK:CVR company-ID schemes.
func applyParty(p *ubl.Party) {
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
		applyTaxScheme(pts.TaxScheme)
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

// applyTaxRepParty drops the elements OIOUBL forbids on a cac:TaxRepresentativeParty
// (EndpointID, PartyIdentification, PartyLegalEntity, Contact) before the standard pass.
func applyTaxRepParty(p *ubl.Party) {
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
