package addon

import (
	"maps"
	"strings"

	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

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
	// icd is the code that names this register inside the participant
	// identifier scheme. The 0xxx codes are ISO 6523 ICDs; the 9xxx ones are
	// Peppol's own allocations. Every register OIOUBL accepts has one, so
	// every endpoint has the same shape.
	icd cbc.Code

	// prefix is what the "DK" prefix does to this register's code.
	prefix prefixRule
}

// registers maps each OIOUBL register to the code that names it in a
// participant identifier, from the Peppol participant identifier scheme code
// list v9.7. Where Peppol has re-coded a register the live code is used; where
// it has retired one without re-coding it, the code it retired is kept, since
// it is still the only one that names that register and Peppol has not reused
// it. Retired here means Peppol will not route the address, which was already
// true of these registers -- it does not make the identifier ambiguous.
var registers = map[cbc.Code]register{
	"GLN":  {icd: "0088"},
	"DUNS": {icd: "0060"},
	"IBAN": {icd: "9918"},

	"DK:P":   {icd: "0096"},
	"DK:CVR": {icd: "0184", prefix: prefixWire}, // Peppol "DK:DIGST"
	"DK:SE":  {icd: "0198", prefix: prefixCode}, // Peppol "DK:ERST"
	// CPR is a personal number and VANS a Danish-only routing register, so
	// Peppol removed both and re-coded neither. OIOUBL still accepts them.
	"DK:CPR":  {icd: "9901"},
	"DK:VANS": {icd: "9905"},

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
	"IS:KT":     {icd: "0196"}, // Peppol "IS:KTNR", the same kennitala
	"EU:REID":   {icd: "9913"},
	// The Finnish registers were removed, not re-coded: Peppol's live 0216
	// carries the "0037" prefix inside the value, so it names a differently
	// spelled identifier rather than this one.
	"FI:OVT":   {icd: "0037"},
	"FI:ORGNR": {icd: "0212"}, // Peppol "FI:ORG"

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
	// Removed without a successor naming the same register.
	"FI:VAT": {icd: "0213"}, "SE:VAT": {icd: "9955"},
}

// recodedICDs are the codes Peppol replaced when it re-coded a register, each
// mapped onto the register it named. Nothing is written under them, but an
// identifier stored before the re-coding, or sent by someone who has not caught
// up, still names a register we recognise and settles on the live code.
var recodedICDs = map[cbc.Code]cbc.Code{
	"9902": RegisterDKCVR,
	"9904": RegisterDKSE,
	"9906": "IT:VAT",
	"9907": "IT:CF",
	"9908": "NO:ORGNR",
	"9917": "IS:KT",
	"9921": "IT:IPA",
}

// registerByICD indexes the registers by the code that names them, the codes
// Peppol has replaced included, so one address has one reading whichever of
// them it arrived under.
var registerByICD = buildRegisterByICD()

func buildRegisterByICD() map[cbc.Code]cbc.Code {
	index := make(map[cbc.Code]cbc.Code, len(registers)+len(recodedICDs))
	for name, reg := range registers {
		index[reg.icd] = name
	}
	maps.Copy(index, recodedICDs)
	return index
}

// OIOUBLEndpointURI builds the participant identifier URI of an address in the
// given OIOUBL register, settling the code on what that register expects, as
// "iso6523-actorid-upis::0184:12345674". The scheme is GOBL's iso.ActorIDScheme
// and is not copied here: it is Peppol's, and NemHandel shares it, the
// Nemhandelsregister being itself a Peppol SMP -- so a Danish party is the same
// participant on both networks and is spelled the same way in both. Every
// endpoint this addon writes takes this shape; there is no second one.
//
// The URI is empty when the register is not one OIOUBL names, or when nothing
// addressable is left, such as a code that was only the "DK" prefix.
func OIOUBLEndpointURI(name, code cbc.Code) cbc.URI {
	reg, ok := registers[name]
	if !ok {
		return ""
	}
	if code = normalizeEndpointCode(name, code); code == cbc.CodeEmpty {
		return ""
	}
	// Peppol's own "<scheme>::<register>:<code>" serialisation: the doubled
	// colon is the separator, not an empty segment.
	return cbc.URI(iso.ActorIDScheme + "::" + reg.icd.String() + ":" + code.String())
}

// SplitEndpointURI returns the register, as OIOUBL spells it, and the code of a
// participant identifier URI; ok is false for any other network. Two spellings
// are read: the one this addon writes, and the bare "DK:CVR:12345674" stored
// before it.
func SplitEndpointURI(uri cbc.URI) (name, code cbc.Code, ok bool) {
	if uri.Scheme() == iso.ActorIDScheme {
		return splitICDEndpoint(uri.Opaque())
	}
	return splitRegisterEndpoint(uri.String())
}

// splitICDEndpoint reads "<register>:<code>" from the scheme-specific part,
// which Peppol's serialisation leaves with the separator's second colon at its
// head.
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

// splitRegisterEndpoint reads the earlier spelling's "<register>:<code>", where
// the register itself holds a colon in most cases, so the last one separates it
// from the code.
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

// normalizeEndpointCode settles a code on what its register expects. OIOUBL
// writes "DK" on both Danish numbers in the XML, but the two identifiers
// differ: 0184 is the bare CVR, 0198 carries the prefix.
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
// the uppercase spelling the register and the XML both want.
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
