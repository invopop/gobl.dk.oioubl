package addon

import (
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// partyRules validates the OIOUBL address format declared on a party via
// dk-oioubl-address-format (GOBL has no address-level extension). An absent
// format serializes as StructuredLax, which imposes no completeness requirement.
func partyRules() *rules.Set {
	return rules.For(new(org.Party),
		rules.Field("ext",
			rules.AssertIfPresent("36", "address format must be an OIOUBL addressformatcode-1.1 value (F-LIB027)",
				tax.ExtensionHasValidCode(ExtKeyAddressFormat)),
		),
		rules.When(is.Func("party with a declared address format", partyHasAddressFormat),
			rules.Assert("37", "the party address is incomplete for its declared OIOUBL address format (F-LIB031 / F-LIB033 / F-LIB034 / F-LIB035 / F-LIB037 / F-LIB039)",
				is.Func("address satisfies the declared format", partyAddressFormatComplete)),
		),
	)
}

func partyHasAddressFormat(val any) bool {
	p, ok := val.(*org.Party)
	return ok && p != nil && p.Ext.Get(ExtKeyAddressFormat) != ""
}

// partyAddressFormatComplete reports whether a party's first postal address
// carries the data OIOUBL requires for its declared addressformatcode-1.1 value.
func partyAddressFormatComplete(val any) bool {
	p, ok := val.(*org.Party)
	if !ok || p == nil {
		return true
	}
	var addr *org.Address
	if len(p.Addresses) > 0 {
		addr = p.Addresses[0]
	}
	switch p.Ext.Get(ExtKeyAddressFormat) {
	case ExtValueAddressFormatStructuredDK:
		// F-LIB033 PostalZone; F-LIB034 StreetName or Postbox; F-LIB035
		// BuildingNumber or Postbox.
		if addr == nil || addr.Code == "" {
			return false
		}
		if addr.Street == "" && addr.PostOfficeBox == "" {
			return false
		}
		return addr.Number != "" || addr.PostOfficeBox != ""
	case ExtValueAddressFormatUnstructured:
		// F-LIB031 allows only AddressLine, so there must be free-text content to
		// render into it.
		return addr != nil && (addr.Street != "" || addr.StreetExtra != "" || addr.PostOfficeBox != "")
	case ExtValueAddressFormatStructuredID:
		// F-LIB037: the register identifier is mandatory. GOBL has no
		// address-identifier field, so the GLN rides org.Address.Number (emitted as
		// cbc:ID).
		return addr != nil && addr.Number != ""
	case ExtValueAddressFormatStructuredRegion:
		// F-LIB039: region, district or country is required. The district maps to
		// org.Address.Locality ("village, town, district, or city").
		return addr != nil && (addr.Region != "" || addr.Locality != "" || addr.Country != "")
	}
	// StructuredLax imposes no completeness requirement.
	return true
}
