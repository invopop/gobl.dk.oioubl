package oioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

// addressStructuredLax is the OIOUBL addressformatcode-1.1 value emitted on
// every outbound address (no mandatory sub-fields).
const addressStructuredLax = "StructuredLax"

// firstAddress is the one address OIOUBL maps, since UBL has room for a single
// postal address where GOBL allows several.
func firstAddress(addresses []*org.Address) *org.Address {
	if len(addresses) == 0 {
		return nil
	}
	return addresses[0]
}

// newPostalAddress builds an OIOUBL address from scratch, for the places the
// base leaves one out entirely. Nil when there is nothing to build it from.
func newPostalAddress(addresses []*org.Address) *ubl.PostalAddress {
	a := firstAddress(addresses)
	if a == nil {
		return nil
	}
	addr := new(ubl.PostalAddress)
	applyAddress(addr, a)
	return addr
}

// applyAddress writes a GOBL address onto a UBL one, dropping the forbidden
// LocationCoordinate (F-LIB212) and stamping the mandatory AddressFormatCode (F-LIB025).
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
		addr.StreetName = ptr(a.Street)
	}
	if a.Number != "" {
		addr.BuildingNumber = ptr(a.Number)
	}
	if a.PostOfficeBox != "" {
		addr.Postbox = ptr(a.PostOfficeBox)
	}
	if a.StreetExtra != "" {
		addr.AdditionalStreetName = ptr(a.LineTwo())
	}
	if a.Locality != "" {
		addr.CityName = ptr(a.Locality)
	}
	if a.Region != "" {
		addr.CountrySubentity = ptr(a.Region)
	}
	if a.Code != cbc.CodeEmpty {
		addr.PostalZone = ptr(a.Code.String())
	}
	if a.Country != "" {
		addr.Country = &ubl.Country{IdentificationCode: string(a.Country)}
	}
}

// newAddressFormatCode builds the cbc:AddressFormatCode required on every OIOUBL address (F-LIB025).
func newAddressFormatCode(value string) *ubl.IDType {
	return &ubl.IDType{
		ListID:       ptr(codelistAddressFormat),
		ListAgencyID: ptr(agencyID),
		Value:        value,
	}
}
