package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"strings"

	oioubl "github.com/invopop/gobl.dk.oioubl/addon"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
)

func goblParty(party *Party) *org.Party {
	if party == nil {
		return nil
	}
	p := &org.Party{}

	if party.PartyLegalEntity != nil && party.PartyLegalEntity.RegistrationName != nil {
		p.Name = ubl.CleanString(*party.PartyLegalEntity.RegistrationName)
	}

	if eID := party.EndpointID; eID != nil {
		if eID.SchemeID == ubl.SchemeIDEmail {
			p.Inboxes = append(p.Inboxes, &org.Inbox{Email: eID.Value})
		} else {
			// Restored as org.Endpoints (org.Inbox is deprecated); scheme and code
			// round-trip verbatim, reversing only the wire-only DK prefix (F-LIB180).
			code := eID.Value
			if eID.SchemeID == schemeDKCVR || eID.SchemeID == schemeDKSE {
				code = strings.TrimPrefix(code, "DK")
			}
			p.Endpoints = append(p.Endpoints, &org.Endpoint{
				URI: cbc.URI(oioubl.OIOUBLEndpointURI(eID.SchemeID, code)),
			})
		}
	}

	if party.PartyName != nil {
		if p.Name == "" {
			p.Name = ubl.CleanString(party.PartyName.Name)
		} else if party.PartyName.Name != p.Name {
			p.Alias = ubl.CleanString(party.PartyName.Name)
		}
	}

	if c := party.Contact; c != nil {
		person := new(org.Person)
		if c.Name != nil {
			person.Name = &org.Name{
				Given: ubl.CleanString(*c.Name),
			}
		}
		// Restore cac:Contact/cbc:ID to the person's identities for a lossless
		// round-trip (the outbound side sources it from there for F-INV051).
		if c.ID != nil {
			if code := ubl.CleanString(*c.ID); code != "" {
				person.Identities = []*org.Identity{{Code: cbc.Code(code)}}
			}
		}
		if person.Name != nil || len(person.Identities) > 0 {
			p.People = []*org.Person{person}
		}
	}

	if party.PostalAddress != nil {
		p.Addresses = []*org.Address{
			parseAddress(party.PostalAddress),
		}
	}

	if party.Contact != nil {
		if party.Contact.Telephone != nil {
			p.Telephones = []*org.Telephone{
				{
					Number: ubl.CleanString(*party.Contact.Telephone),
				},
			}
		}
		if party.Contact.ElectronicMail != nil {
			p.Emails = []*org.Email{
				{
					Address: ubl.CleanString(*party.Contact.ElectronicMail),
				},
			}
		}
	}

	handleLegalEntityIdentity(party, p)
	handlePartyTaxSchemes(party, p)
	handlePartyIdentifications(party, p)

	return p
}

func goblDeliveryParty(party *Party) *org.Party {
	if party == nil {
		return nil
	}
	p := &org.Party{}

	if party.PartyLegalEntity != nil && party.PartyLegalEntity.RegistrationName != nil {
		p.Name = ubl.CleanString(*party.PartyLegalEntity.RegistrationName)
	}
	if party.PartyName != nil {
		if p.Name == "" {
			p.Name = ubl.CleanString(party.PartyName.Name)
		}
	}

	if p.Name == "" {
		return nil
	}
	return p
}

func parseAddress(address *PostalAddress) *org.Address {
	if address == nil {
		return nil
	}

	addr := new(org.Address)
	if address.Country != nil {
		addr.Country = l10n.ISOCountryCode(address.Country.IdentificationCode)
	}
	if address.StreetName != nil {
		addr.Street = ubl.CleanString(*address.StreetName)
	}
	if address.AdditionalStreetName != nil {
		addr.StreetExtra = ubl.CleanString(*address.AdditionalStreetName)
	}
	if address.CityName != nil {
		addr.Locality = ubl.CleanString(*address.CityName)
	}
	if address.PostalZone != nil {
		addr.Code = cbc.Code(ubl.CleanString(*address.PostalZone))
	}
	if address.CountrySubentity != nil {
		addr.Region = ubl.CleanString(*address.CountrySubentity)
	}
	// A StructuredRegion address carries its region in cbc:Region (F-LIB040).
	if address.Region != nil && addr.Region == "" {
		addr.Region = ubl.CleanString(*address.Region)
	}
	// A StructuredRegion address carries the locality in cbc:District (F-LIB040).
	if address.District != nil && addr.Locality == "" {
		addr.Locality = ubl.CleanString(*address.District)
	}
	if address.BuildingNumber != nil {
		addr.Number = ubl.CleanString(*address.BuildingNumber)
	}
	// A StructuredID address is just a register identifier (a GLN) in cbc:ID
	// (F-LIB037/038). GOBL has no such field, so it rides org.Address.Number
	// (idle in this format); the emit side re-reads it from there.
	if address.AddressFormatCode != nil &&
		address.AddressFormatCode.Value == addressStructuredID &&
		address.ID != nil {
		addr.Number = ubl.CleanString(address.ID.Value)
	}
	if address.Postbox != nil {
		addr.PostOfficeBox = ubl.CleanString(*address.Postbox)
	}
	// CitySubdivisionName maps to StreetExtra in GOBL.
	if address.CitySubdivisionName != nil && addr.StreetExtra == "" {
		addr.StreetExtra = ubl.CleanString(*address.CitySubdivisionName)
	}
	// Unstructured addresses carry content as free-text cac:AddressLine: fall
	// back to it, first line as street and the rest as street extra.
	if addr.Street == "" && len(address.AddressLine) > 0 {
		var lines []string
		for _, l := range address.AddressLine {
			if s := ubl.CleanString(l.Line); s != "" {
				lines = append(lines, s)
			}
		}
		if len(lines) > 0 {
			addr.Street = lines[0]
			if len(lines) > 1 && addr.StreetExtra == "" {
				addr.StreetExtra = strings.Join(lines[1:], ", ")
			}
		}
	}
	return addr
}

func handleLegalEntityIdentity(party *Party, p *org.Party) {
	if party.PartyLegalEntity == nil || party.PartyLegalEntity.CompanyID == nil {
		return
	}
	if p.Identities == nil {
		p.Identities = make([]*org.Identity, 0)
	}
	identity := &org.Identity{
		Code:  cbc.Code(party.PartyLegalEntity.CompanyID.Value),
		Scope: org.IdentityScopeLegal,
	}
	if party.PartyLegalEntity.CompanyID.SchemeID != nil {
		identity.Ext = tax.ExtensionsOf(cbc.CodeMap{
			iso.ExtKeySchemeID: cbc.Code(*party.PartyLegalEntity.CompanyID.SchemeID),
		})
	}
	p.Identities = append(p.Identities, identity)
}

// OIOUBL: resolves the country via resolveCountry, falling back to the Danish company-ID scheme when the address has none (F-LIB038).
func handlePartyTaxSchemes(party *Party, p *org.Party) {
	if len(party.PartyTaxScheme) == 0 {
		return
	}

	cc := resolveCountry(party)
	validSchemes := extractValidTaxSchemes(party.PartyTaxScheme)

	if len(validSchemes) == 1 {
		setTaxIDFromScheme(validSchemes[0], p, cc)
	} else if len(validSchemes) > 1 {
		handleMultipleTaxSchemes(validSchemes, p, cc)
	}
}

// resolveCountry falls back to the DK company-ID scheme when a StructuredID address carries no country (F-LIB038).
func resolveCountry(p *Party) string {
	if c := p.CountryCode(); c != "" {
		return c
	}
	if hasDanishCompanyScheme(p) {
		return "DK"
	}
	return ""
}

// hasDanishCompanyScheme reports whether any company ID carries a Danish OIOUBL scheme (DK:SE/DK:CVR).
func hasDanishCompanyScheme(p *Party) bool {
	for _, pts := range p.PartyTaxScheme {
		if id := pts.CompanyID; id != nil && id.SchemeID != nil &&
			(*id.SchemeID == schemeDKSE || *id.SchemeID == schemeDKCVR) {
			return true
		}
	}
	if le := p.PartyLegalEntity; le != nil && le.CompanyID != nil && le.CompanyID.SchemeID != nil &&
		(*le.CompanyID.SchemeID == schemeDKSE || *le.CompanyID.SchemeID == schemeDKCVR) {
		return true
	}
	return false
}

func extractValidTaxSchemes(schemes []PartyTaxScheme) []PartyTaxScheme {
	validSchemes := make([]PartyTaxScheme, 0)
	for _, pts := range schemes {
		if pts.CompanyID != nil && pts.CompanyID.Value != "" && pts.TaxScheme != nil {
			validSchemes = append(validSchemes, pts)
		}
	}
	return validSchemes
}

// OIOUBL: maps the 63/Moms tax scheme back via goblTaxSchemeCategory.
func setTaxIDFromScheme(pts PartyTaxScheme, p *org.Party, countryCode string) {
	p.TaxID = &tax.Identity{
		Country: l10n.TaxCountryCode(countryCode),
		Code:    cbc.Code(pts.CompanyID.Value),
	}
	sc := goblTaxSchemeCategory(pts.TaxScheme.ID.Value)
	if p.TaxID.GetScheme() != sc {
		var scheme cbc.Code
		if pts.TaxScheme.TaxTypeCode != nil && pts.TaxScheme.TaxTypeCode.Value != "" {
			scheme = cbc.Code(pts.TaxScheme.TaxTypeCode.Value)
		} else {
			scheme = sc
		}
		p.TaxID.Scheme = scheme
	}
}

func handleMultipleTaxSchemes(validSchemes []PartyTaxScheme, p *org.Party, countryCode string) {
	// Multiple tax schemes: look for VAT, otherwise use first
	vatIdx := findVATSchemeIndex(validSchemes)

	taxIDIdx := 0
	if vatIdx != -1 {
		taxIDIdx = vatIdx
	}

	setTaxIDFromScheme(validSchemes[taxIDIdx], p, countryCode)
	addRemainingTaxSchemesAsIdentities(validSchemes, taxIDIdx, p, countryCode)
}

// OIOUBL: matches VAT via goblTaxSchemeCategory, which maps the 63/Moms scheme.
func findVATSchemeIndex(schemes []PartyTaxScheme) int {
	for i, pts := range schemes {
		if goblTaxSchemeCategory(pts.TaxScheme.ID.Value) == cbc.Code(ubl.TaxSchemeVAT) {
			return i
		}
	}
	return -1
}

// OIOUBL: maps each identity's tax scheme back via goblTaxSchemeCategory.
func addRemainingTaxSchemesAsIdentities(validSchemes []PartyTaxScheme, taxIDIdx int, p *org.Party, countryCode string) {
	for i, pts := range validSchemes {
		if i == taxIDIdx {
			continue
		}

		identity := &org.Identity{
			Country: l10n.ISOCountryCode(countryCode),
			Code:    cbc.Code(pts.CompanyID.Value),
			Scope:   org.IdentityScopeTax,
			Type:    goblTaxSchemeCategory(pts.TaxScheme.ID.Value),
		}

		if p.Identities == nil {
			p.Identities = make([]*org.Identity, 0)
		}
		p.Identities = append(p.Identities, identity)
	}
}

// OIOUBL: reverses the wire-only DK prefix on DK:CVR/DK:SE identities (F-LIB180).
func handlePartyIdentifications(party *Party, p *org.Party) {
	for _, partyID := range party.PartyIdentification {
		if partyID.ID != nil {
			code := partyID.ID.Value
			identity := &org.Identity{}
			if partyID.ID.SchemeID != nil {
				s := *partyID.ID.SchemeID
				identity.Ext = tax.ExtensionsOf(cbc.CodeMap{
					iso.ExtKeySchemeID: cbc.Code(s),
				})
				if s == schemeDKCVR || s == schemeDKSE {
					// Reverse the wire-only DK prefix (F-LIB180).
					code = strings.TrimPrefix(code, "DK")
				}
			}
			identity.Code = cbc.Code(code)
			if p.Identities == nil {
				p.Identities = make([]*org.Identity, 0)
			}
			p.Identities = append(p.Identities, identity)
		}
	}
}
