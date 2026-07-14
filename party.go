package dkoioubl

import (
	"fmt"
	ubl "github.com/invopop/gobl.ubl"
	"strings"

	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

func newParty(party *org.Party) *Party {
	if party == nil {
		return nil
	}
	p := &Party{
		PostalAddress: newAddress(party.Addresses),
	}
	if party.Name != "" {
		p.PartyName = &PartyName{Name: party.Name}
		p.PartyLegalEntity = &PartyLegalEntity{RegistrationName: &party.Name}
	}
	addPartyTaxScheme(p, party)
	p.Contact = newPartyContact(party)
	addPartyEndpoint(p, party)
	if party.Alias != "" {
		p.PartyName = &PartyName{Name: party.Alias}
	}
	addPartyIdentities(p, party)
	return p
}

// addPartyTaxScheme maps the primary tax identity to a PartyTaxScheme and stamps
// its country onto the postal address.
func addPartyTaxScheme(p *Party, party *org.Party) {
	tID := party.TaxID
	if tID == nil || tID.Code == "" {
		return
	}
	code := tID.String()
	id := tID.GetScheme()
	if id == cbc.CodeEmpty {
		id = ubl.TaxSchemeVAT
	}
	p.PartyTaxScheme = []PartyTaxScheme{{
		CompanyID: &IDType{Value: code},
		TaxScheme: &TaxScheme{ID: IDType{Value: id.String()}},
	}}
	if p.PostalAddress == nil {
		p.PostalAddress = new(PostalAddress)
	}
	p.PostalAddress.Country = &Country{IdentificationCode: tID.Country.String()}
}

// newPartyContact builds the cac:Contact, returning nil when empty. The
// mandatory cbc:ID (F-INV051) comes from the person's identity, not fabricated.
func newPartyContact(party *org.Party) *Contact {
	contact := &Contact{}
	if len(party.Emails) > 0 {
		contact.ElectronicMail = &party.Emails[0].Address
	}
	if len(party.Telephones) > 0 {
		contact.Telephone = &party.Telephones[0].Number
	}
	if len(party.People) > 0 {
		if n := contactName(party.People[0].Name); n != "" {
			contact.Name = &n
		}
		if ids := party.People[0].Identities; len(ids) > 0 && ids[0].Code != "" {
			code := ids[0].Code.String()
			contact.ID = &code
		}
	}
	if contact.Name == nil && contact.Telephone == nil && contact.ElectronicMail == nil && contact.ID == nil {
		return nil
	}
	return contact
}

// addPartyEndpoint derives the cbc:EndpointID from the OIOUBL endpoint URI
// (scheme+code emitted 1:1), falling back to the first inbox.
func addPartyEndpoint(p *Party, party *org.Party) {
	for _, ep := range party.Endpoints {
		if ep == nil {
			continue
		}
		if scheme, value, ok := parseOIOUBLEndpoint(ep.URI.String()); ok {
			p.EndpointID = &EndpointID{SchemeID: scheme.String(), Value: value.String()}
			break
		}
	}
	if p.EndpointID == nil && len(party.Inboxes) > 0 {
		ib := party.Inboxes[0]
		if ib.Email != "" {
			p.EndpointID = &EndpointID{SchemeID: ubl.SchemeIDEmail, Value: ib.Email}
		} else if ib.Scheme != "" {
			p.EndpointID = &EndpointID{SchemeID: ib.Scheme.String(), Value: ib.Code.String()}
		}
	}
}

// addPartyIdentities classifies identities: first legal-scope →
// PartyLegalEntity.CompanyID, tax-scope → PartyTaxScheme, rest → PartyIdentification.
func addPartyIdentities(p *Party, party *org.Party) {
	firstLegalIdx := -1
	for i, id := range party.Identities {
		if id.Scope != org.IdentityScopeLegal {
			continue
		}
		if p.PartyLegalEntity == nil {
			p.PartyLegalEntity = &PartyLegalEntity{}
		}
		p.PartyLegalEntity.CompanyID = &IDType{Value: id.Code.String()}
		if s := id.Ext.Get(iso.ExtKeySchemeID).String(); s != "" {
			p.PartyLegalEntity.CompanyID.SchemeID = &s
		}
		firstLegalIdx = i
		break
	}
	for _, id := range party.Identities {
		if id.Scope != org.IdentityScopeTax {
			continue
		}
		companyID := &IDType{Value: id.Code.String()}
		if s := id.Ext.Get(iso.ExtKeySchemeID).String(); s != "" {
			companyID.SchemeID = &s
		}
		p.PartyTaxScheme = append(p.PartyTaxScheme, PartyTaxScheme{
			CompanyID: companyID,
			TaxScheme: &TaxScheme{ID: IDType{Value: id.Type.String()}},
		})
	}
	for i, id := range party.Identities {
		if (id.Scope == org.IdentityScopeLegal && i == firstLegalIdx) || id.Scope == org.IdentityScopeTax {
			continue
		}
		idType := &IDType{Value: id.Code.String()}
		if s := id.Ext.Get(iso.ExtKeySchemeID).String(); s != "" {
			idType.SchemeID = &s
		} else if id.Ext.IsZero() {
			if t := id.Type.String(); t != "" {
				idType.SchemeID = &t
			}
		}
		p.PartyIdentification = append(p.PartyIdentification, Identification{ID: idType})
	}
}

func newDeliveryParty(party *org.Party) *Party {
	if party == nil {
		return nil
	}

	p := &Party{}
	hasContent := false

	if party.Name != "" {
		p.PartyName = &PartyName{
			Name: party.Name,
		}
		p.PartyLegalEntity = &PartyLegalEntity{
			RegistrationName: &party.Name,
		}
		hasContent = true
	}

	// No PostalAddress per UBL-CR-394 (address lives in DeliveryLocation).

	contact := &Contact{}

	if len(party.Emails) > 0 {
		contact.ElectronicMail = &party.Emails[0].Address
	}

	if len(party.Telephones) > 0 {
		contact.Telephone = &party.Telephones[0].Number
	}

	if len(party.People) > 0 {
		n := contactName(party.People[0].Name)
		if n != "" {
			contact.Name = &n
		}
	}

	if contact.Name != nil || contact.Telephone != nil || contact.ElectronicMail != nil {
		p.Contact = contact
		hasContent = true
	}

	// Return nil when empty to avoid emitting an empty XML element.
	if !hasContent {
		return nil
	}

	return p
}

func newPayeeParty(party *org.Party) *Party {
	if party == nil {
		return nil
	}
	p := &Party{
		PartyName: &PartyName{
			Name: party.Name,
		},
	}

	// First identity with a valid scheme only (UBL-SR-20: maximum once),
	// preferring an Ext schemeID or a 4-digit ISO 6523 ICD label.
	if len(party.Identities) > 0 {
		for _, id := range party.Identities {
			var schemeID *string
			if s := id.Ext.Get(iso.ExtKeySchemeID).String(); s != "" {
				schemeID = &s
			}
			if schemeID == nil && id.Label != "" && len(id.Label) == 4 {
				schemeID = &id.Label
			}
			if schemeID != nil {
				code := id.Code.String()
				p.PartyIdentification = []Identification{
					{ID: &IDType{
						Value:    code,
						SchemeID: schemeID,
					}},
				}
				break
			}
		}
	}

	// PartyLegalEntity carries the legal identity's CompanyID but no RegistrationName (UBL-CR-275).
	for _, id := range party.Identities {
		if id.Scope == org.IdentityScopeLegal {
			code := id.Code.String()
			p.PartyLegalEntity = &PartyLegalEntity{
				CompanyID: &IDType{
					Value: code,
				},
			}
			if s := id.Ext.Get(iso.ExtKeySchemeID).String(); s != "" {
				p.PartyLegalEntity.CompanyID.SchemeID = &s
			}
			break
		}
	}

	return p
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

func newAddress(addresses []*org.Address) *PostalAddress {
	if len(addresses) == 0 {
		return nil
	}
	a := addresses[0]

	addr := &PostalAddress{}

	// Every OIOUBL address needs an AddressFormatCode (F-LIB025); stamping it
	// here also covers delivery and payee addresses, not just supplier/customer.
	addr.AddressFormatCode = newAddressFormatCode(addressStructuredLax)
	if a.Street != "" {
		addr.StreetName = &a.Street
	}
	if a.Number != "" {
		addr.BuildingNumber = &a.Number
	}
	if a.PostOfficeBox != "" {
		addr.Postbox = &a.PostOfficeBox
	}

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

	if a.Block != "" {
		addr.PlotIdentification = &a.Block
	}

	if a.Country != "" {
		addr.Country = &Country{IdentificationCode: string(a.Country)}
	}

	// OIOUBL forbids cac:LocationCoordinate on an address (F-LIB212).

	return addr
}

func contactName(n *org.Name) string {
	given := n.Given
	surname := n.Surname

	if given == "" && surname == "" {
		return ""
	}
	if given == "" {
		return surname
	}
	if surname == "" {
		return given
	}

	return fmt.Sprintf("%s %s", given, surname)
}

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
