package addon

import (
	"maps"
	"strings"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

// EndpointScheme is the URI scheme of a participant identifier: the ISO 6523
// ICD and the code follow it, as "iso6523-actorid-upis::0184:12345674". It is
// Peppol's scheme and NemHandel shares it -- the Nemhandelsregister is itself
// a Peppol SMP, so a Danish party is the same participant on both networks and
// is spelled the same way in both.
const EndpointScheme = "iso6523-actorid-upis"

// RegisterEndpointScheme names the OIOUBL register in place of an ICD, as
// "nemhandel:dk:vans:12345674". It carries the few registers OIOUBL accepts
// that Peppol has retired without a successor, so that an address in one of
// them still has a URI to live in.
const RegisterEndpointScheme = "nemhandel"

// prefixRule says what OIOUBL's "DK" prefix means for a register's code.
type prefixRule uint8

const (
	// prefixNone keeps the code exactly as given. Most registers' codes carry
	// letters that mean something, so nothing is added or taken away.
	prefixNone prefixRule = iota

	// prefixWire means OIOUBL writes "DK" in the XML but the identifier itself
	// is the bare number: ICD 0184 is `[1-9][0-9]{7}`, so the prefix is added
	// on the way out (F-LIB180) and never stored.
	prefixWire

	// prefixCode means the "DK" belongs to the identifier: ICD 0198 is
	// `DK[0-9]{8}`, and OIOUBL spells the value the same way (F-LIB184,
	// F-LIB196). It is stored with the prefix and needs none added.
	prefixCode
)

// register is one entry of the list an OIOUBL cbc:EndpointID may name
// (F-LIB179, schematron 1.17.2; invoices and responses share the list).
type register struct {
	// icd is the ISO 6523 ICD Peppol assigns to this same register today. It
	// is empty where Peppol has retired the register with no successor, or
	// where the successor's value is spelled differently enough that mapping
	// onto it would change the identifier.
	icd cbc.Code

	// prefix is what the "DK" prefix does to this register's code.
	prefix prefixRule
}

// registers maps each OIOUBL register to the participant identifier scheme that
// names the same register. The ICDs are those of the Peppol participant
// identifier scheme code list v9.7; a register left without one is written
// under RegisterEndpointScheme instead.
var registers = map[cbc.Code]register{
	"GLN":  {icd: "0088"},
	"DUNS": {icd: "0060"},
	"IBAN": {icd: "9918"},

	"DK:P":   {icd: "0096"},
	"DK:CVR": {icd: "0184", prefix: prefixWire}, // Peppol "DK:DIGST"
	"DK:SE":  {icd: "0198", prefix: prefixCode}, // Peppol "DK:ERST"
	// CPR is a personal number and VANS a Danish-only routing register;
	// Peppol removed both (9901, 9905) and named no successor.
	"DK:CPR":  {},
	"DK:VANS": {},

	"FR:SIRET":  {icd: "0009"},
	"SE:ORGNR":  {icd: "0007"},
	"IT:FTI":    {icd: "0097"},
	"IT:SIA":    {icd: "0135"},
	"IT:SECETI": {icd: "0142"},
	"IT:CF":     {icd: "0210"}, // Peppol "IT:CFI", the same codice fiscale
	"IT:IPA":    {icd: "0201"}, // Peppol "IT:CUUO", the same iPA office code
	"NO:ORGNR":  {icd: "0192"}, // Peppol "NO:ORG", the same organisasjonsnummer
	"AT:GOV":    {icd: "9915"},
	"AT:CID":    {icd: "9916"},
	"AT:KUR":    {icd: "9919"},
	"IS:KT":     {icd: "9917"},
	"EU:REID":   {icd: "9913"},
	// Peppol's live Finnish codes are not these registers: 0216 carries the
	// "0037" prefix inside the value, and 0212/0213 were removed outright.
	"FI:OVT":   {},
	"FI:ORGNR": {},

	"AD:VAT": {icd: "9922"}, "AL:VAT": {icd: "9923"}, "AT:VAT": {icd: "9914"},
	"BA:VAT": {icd: "9924"}, "BE:VAT": {icd: "9925"}, "BG:VAT": {icd: "9926"},
	"CH:VAT": {icd: "9927"}, "CY:VAT": {icd: "9928"}, "CZ:VAT": {icd: "9929"},
	"DE:VAT": {icd: "9930"}, "EE:VAT": {icd: "9931"}, "ES:VAT": {icd: "9920"},
	"EU:VAT": {icd: "9912"}, "GB:VAT": {icd: "9932"}, "GR:VAT": {icd: "9933"},
	"HR:VAT": {icd: "9934"}, "HU:VAT": {icd: "9910"}, "IE:VAT": {icd: "9935"},
	"IT:VAT": {icd: "0211"}, // Peppol "IT:IVA", the same partita IVA
	"LI:VAT": {icd: "9936"}, "LT:VAT": {icd: "9937"}, "LU:VAT": {icd: "9938"},
	"LV:VAT": {icd: "9939"}, "MC:VAT": {icd: "9940"}, "ME:VAT": {icd: "9941"},
	"MK:VAT": {icd: "9942"}, "MT:VAT": {icd: "9943"}, "NL:VAT": {icd: "9944"},
	"NO:VAT": {icd: "9909"}, "PL:VAT": {icd: "9945"}, "PT:VAT": {icd: "9946"},
	"RO:VAT": {icd: "9947"}, "RS:VAT": {icd: "9948"}, "SI:VAT": {icd: "9949"},
	"SK:VAT": {icd: "9950"}, "SM:VAT": {icd: "9951"}, "TR:VAT": {icd: "9952"},
	"VA:VAT": {icd: "9953"},
	// Peppol removed both without a successor of the same register.
	"FI:VAT": {}, "SE:VAT": {},
}

// retiredICDs are the ICDs Peppol has removed, mapped onto the register they
// named. Nothing is written under them, but a participant identifier stored
// before the re-coding still names a register we recognise, and one arriving
// from a sender that has not caught up is read rather than refused.
var retiredICDs = map[cbc.Code]cbc.Code{
	"0037": "FI:OVT",
	"0212": "FI:ORGNR",
	"0213": "FI:VAT",
	"9901": "DK:CPR",
	"9902": RegisterDKCVR,
	"9904": RegisterDKSE,
	"9905": "DK:VANS",
	"9906": "IT:VAT",
	"9907": "IT:CF",
	"9908": "NO:ORGNR",
	"9921": "IT:IPA",
	"9955": "SE:VAT",
}

// registerByICD indexes the registers by the ICD that names them, retired
// codes included, so one address has one reading whichever code it arrived in.
var registerByICD = buildRegisterByICD()

func buildRegisterByICD() map[cbc.Code]cbc.Code {
	index := make(map[cbc.Code]cbc.Code, len(registers)+len(retiredICDs))
	for name, reg := range registers {
		if reg.icd != cbc.CodeEmpty {
			index[reg.icd] = name
		}
	}
	maps.Copy(index, retiredICDs)
	return index
}

// OIOUBLEndpointURI builds the participant identifier URI of an address in the
// given OIOUBL register, settling the code on what that register's ICD expects.
// It returns an empty URI when nothing addressable is left, such as a code that
// was only the "DK" prefix.
func OIOUBLEndpointURI(name, code cbc.Code) cbc.URI {
	code = normalizeEndpointCode(name, code)
	if code == cbc.CodeEmpty {
		return ""
	}
	if reg, ok := registers[name]; ok && reg.icd != cbc.CodeEmpty {
		// Peppol's own "<scheme>::<icd>:<code>" serialisation: the doubled
		// colon is the separator, not an empty segment.
		return cbc.URI(EndpointScheme + "::" + reg.icd.String() + ":" + code.String())
	}
	return cbc.URI(RegisterEndpointScheme + ":" + strings.ToLower(name.String()) + ":" + code.String())
}

// SplitEndpointURI returns the register, as OIOUBL spells it, and the code of a
// participant identifier URI; ok is false for any other network. Three
// spellings are read: the ICD form this addon writes, the register form it
// falls back to, and the bare "DK:CVR:12345674" stored before either existed.
func SplitEndpointURI(uri cbc.URI) (name, code cbc.Code, ok bool) {
	if uri.Scheme() == EndpointScheme {
		return splitICDEndpoint(uri.Opaque())
	}
	addr := uri.String()
	if uri.Scheme() == RegisterEndpointScheme {
		addr = uri.Opaque()
	}
	return splitRegisterEndpoint(addr)
}

// splitICDEndpoint reads "<icd>:<code>" from the scheme-specific part, which
// Peppol's serialisation leaves with the separator's second colon at its head.
func splitICDEndpoint(opaque string) (name, code cbc.Code, ok bool) {
	icd, value, found := strings.Cut(strings.TrimPrefix(opaque, ":"), ":")
	if !found || icd == "" || value == "" {
		return "", "", false
	}
	name, ok = registerByICD[cbc.Code(icd)]
	if !ok {
		return "", "", false
	}
	return name, cbc.Code(value), true
}

// splitRegisterEndpoint reads "<register>:<code>", where the register itself
// holds a colon in most cases, so the last one separates it from the code.
func splitRegisterEndpoint(addr string) (name, code cbc.Code, ok bool) {
	i := strings.LastIndex(addr, ":")
	if i <= 0 || i == len(addr)-1 {
		return "", "", false
	}
	name = cbc.Code(strings.ToUpper(addr[:i]))
	if _, ok := registers[name]; !ok {
		return "", "", false
	}
	return name, cbc.Code(addr[i+1:]), true
}

// normalizeEndpointCode settles a code on what its register's ICD expects.
// OIOUBL writes "DK" on both Danish numbers in the XML, but the two
// identifiers differ: ICD 0184 is the bare CVR, ICD 0198 carries the prefix.
func normalizeEndpointCode(name, code cbc.Code) cbc.Code {
	switch registers[name].prefix {
	case prefixNone:
		return code
	case prefixWire:
		return cbc.Code(trimDKPrefix(code.String()))
	case prefixCode:
		bare := trimDKPrefix(code.String())
		if bare == "" {
			return cbc.CodeEmpty
		}
		return cbc.Code("DK" + bare)
	}
	return code
}

// trimDKPrefix removes a leading "DK" in any case: Peppol compares participant
// identifiers case-insensitively, so a lowercased prefix names one address with
// the uppercase spelling the ICD and the XML both want.
func trimDKPrefix(code string) string {
	if len(code) >= 2 && strings.EqualFold(code[:2], "DK") {
		return code[2:]
	}
	return code
}

// OIOUBLEndpoint returns the party's first endpoint naming a register OIOUBL
// accepts (F-LIB179), or nil when it has none; endpoints on another network,
// such as an email or GOBL Net address, are left alone.
func OIOUBLEndpoint(p *org.Party) *org.Endpoint {
	if p == nil {
		return nil
	}
	return oioublEndpoint(p.Endpoints)
}

func oioublEndpoint(eps []*org.Endpoint) *org.Endpoint {
	for _, ep := range eps {
		if ep == nil {
			continue
		}
		if _, _, ok := SplitEndpointURI(ep.URI); ok {
			return ep
		}
	}
	return nil
}

// partyHasOIOUBLEndpoint reports whether at least one endpoint names a
// register OIOUBL accepts. An empty list passes; presence has its own rules.
func partyHasOIOUBLEndpoint(val any) bool {
	eps, ok := val.([]*org.Endpoint)
	if !ok || len(eps) == 0 {
		return true
	}
	return oioublEndpoint(eps) != nil
}
