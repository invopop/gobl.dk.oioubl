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
	goblPartyEndpoint(party, p)
	if party.PartyName != nil {
		if p.Name == "" {
			p.Name = ubl.CleanString(party.PartyName.Name)
		} else if party.PartyName.Name != p.Name {
			p.Alias = ubl.CleanString(party.PartyName.Name)
		}
	}

	goblPartyContact(party, p)
	if party.PostalAddress != nil {
		p.Addresses = []*org.Address{parseAddress(party.PostalAddress)}
	}

	ubl.HandleLegalEntityIdentity(party, p)
	handlePartyTaxSchemes(party, p)
	handlePartyIdentifications(party, p)

	return p
}

// dkUnprefixed reverses the wire-only "DK" prefix OIOUBL mandates on
// DK:CVR/DK:SE identifier values (F-LIB180), leaving other schemes untouched.
func dkUnprefixed(schemeID, code string) string {
	if schemeID == schemeDKCVR || schemeID == schemeDKSE {
		return strings.TrimPrefix(code, "DK")
	}
	return code
}

// goblPartyEndpoint restores the participant identifier as an org.Endpoint
// (org.Inbox is deprecated); scheme and code round-trip verbatim, reversing
// only the wire-only DK prefix (F-LIB180).
func goblPartyEndpoint(party *Party, p *org.Party) {
	eID := party.EndpointID
	if eID == nil {
		return
	}
	if eID.SchemeID == ubl.SchemeIDEmail {
		p.Inboxes = append(p.Inboxes, &org.Inbox{Email: eID.Value})
		return
	}
	code := dkUnprefixed(eID.SchemeID, eID.Value)
	p.Endpoints = append(p.Endpoints, &org.Endpoint{
		URI: oioubl.OIOUBLEndpointURI(cbc.Code(eID.SchemeID), cbc.Code(code)),
	})
}

// goblPartyContact restores the contact person and their telephone/email;
// real documents carry empty <cbc:Telephone/> elements, invalid in GOBL, so
// an empty number is dropped rather than kept.
func goblPartyContact(party *Party, p *org.Party) {
	c := party.Contact
	if c == nil {
		return
	}
	person := new(org.Person)
	if c.Name != nil {
		person.Name = &org.Name{Given: ubl.CleanString(*c.Name)}
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

	if c.Telephone != nil {
		if number := ubl.CleanString(*c.Telephone); number != "" {
			p.Telephones = []*org.Telephone{{Number: number}}
		}
	}
	if c.ElectronicMail != nil {
		p.Emails = []*org.Email{{Address: ubl.CleanString(*c.ElectronicMail)}}
	}
}

// parseAddress builds on gobl.ubl's generic parsing, adding OIOUBL-specific
// fields it doesn't produce: Postbox, StructuredRegion, StructuredID, AddressLine.
func parseAddress(address *PostalAddress) *org.Address {
	if address == nil {
		return nil
	}
	addr := ubl.ParseAddress(address)

	if address.Postbox != nil {
		addr.PostOfficeBox = ubl.CleanString(*address.Postbox)
	}
	// A StructuredRegion address carries its region in cbc:Region (F-LIB040).
	if address.Region != nil && addr.Region == "" {
		addr.Region = ubl.CleanString(*address.Region)
	}
	// A StructuredRegion address carries the locality in cbc:District (F-LIB040).
	if address.District != nil && addr.Locality == "" {
		addr.Locality = ubl.CleanString(*address.District)
	}
	// A StructuredID address's cbc:ID (a GLN, F-LIB037/038) has no GOBL field,
	// so it rides org.Address.Number instead; the emit side re-reads it.
	if address.AddressFormatCode != nil &&
		address.AddressFormatCode.Value == addressStructuredID &&
		address.ID != nil {
		addr.Number = ubl.CleanString(address.ID.Value)
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
				code = dkUnprefixed(s, code)
			}
			identity.Code = cbc.Code(code)
			if p.Identities == nil {
				p.Identities = make([]*org.Identity, 0)
			}
			p.Identities = append(p.Identities, identity)
		}
	}
}
