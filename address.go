package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

// addressStructuredLax is the OIOUBL addressformatcode-1.1 value emitted on
// every outbound address (no mandatory sub-fields).
const addressStructuredLax = "StructuredLax"

// newPostalAddress fills the address fields applyAddress leaves alone. It takes
// the whole slice, like gobl.ubl's own builder, so the "is there one?" check
// lives in one place rather than at every call site.
func newPostalAddress(addresses []*org.Address) *ubl.PostalAddress {
	if len(addresses) == 0 || addresses[0] == nil {
		return nil
	}
	a := addresses[0]
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
