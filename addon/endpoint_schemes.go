package addon

// OIOUBL symbolic EndpointID / company-ID schemes (F-LIB179): numeric ISO 6523
// scheme IDs are rejected on the NemHandel wire, so the gobl.ubl converter maps
// to these symbolic values. DKCVR / DKSE / GLN are the Danish identifiers; ZZZ
// is the OIOUBL "other" scheme, the only company-ID scheme valid for a
// non-Danish identifier (F-LIB189/F-LIB195).
const (
	SchemeDKCVR = "DK:CVR"
	SchemeDKSE  = "DK:SE"
	SchemeGLN   = "GLN"
	SchemeZZZ   = "ZZZ"
)

// EndpointSchemes maps ISO 6523 ICDs / Peppol EAS codes to the symbolic OIOUBL
// EndpointID schemeID codelist (F-LIB179). Only codes with an unambiguous
// symbolic counterpart are listed; anything else passes through numerically.
var EndpointSchemes = map[string]string{
	"0007": "SE:ORGNR",
	"0009": "FR:SIRET",
	"0037": "FI:OVT",
	"0060": "DUNS",
	"0088": SchemeGLN,
	"0096": "DK:P",
	"0184": SchemeDKCVR,
	"0192": "NO:ORGNR",
	"0196": "IS:KT",
	"0198": SchemeDKSE,
	"0212": "FI:ORGNR",
	"0213": "FI:VAT",
	"9902": SchemeDKCVR, // legacy EAS for DK:CVR
	"9906": "IT:VAT",
	"9907": "IT:CF",
	"9909": "NO:VAT",
	"9910": "HU:VAT",
	"9912": "EU:VAT",
	"9913": "EU:REID",
	"9914": "AT:VAT",
	"9915": "AT:GOV",
	"9917": "IS:KT", // legacy EAS for IS:KT
	"9918": "IBAN",
	"9919": "AT:KUR",
	"9920": "ES:VAT",
	"9922": "AD:VAT",
	"9923": "AL:VAT",
	"9924": "BA:VAT",
	"9925": "BE:VAT",
	"9926": "BG:VAT",
	"9927": "CH:VAT",
	"9928": "CY:VAT",
	"9929": "CZ:VAT",
	"9930": "DE:VAT",
	"9931": "EE:VAT",
	"9932": "GB:VAT",
	"9933": "GR:VAT",
	"9934": "HR:VAT",
	"9935": "IE:VAT",
	"9936": "LI:VAT",
	"9937": "LT:VAT",
	"9938": "LU:VAT",
	"9939": "LV:VAT",
	"9940": "MC:VAT",
	"9941": "ME:VAT",
	"9942": "MK:VAT",
	"9943": "MT:VAT",
	"9944": "NL:VAT",
	"9945": "PL:VAT",
	"9946": "PT:VAT",
	"9947": "RO:VAT",
	"9948": "RS:VAT",
	"9949": "SI:VAT",
	"9950": "SK:VAT",
	"9951": "SM:VAT",
	"9952": "TR:VAT",
	"9953": "VA:VAT",
	"9955": "SE:VAT",
}
