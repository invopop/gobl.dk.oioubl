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

// EndpointSchemes maps the Danish ISO 6523 ICDs to their symbolic OIOUBL
// EndpointID schemeID (F-LIB179) — the only schemes the gobl.ubl converter
// derives automatically. A foreign participant carries its scheme explicitly via
// the dk-oioubl-address-scheme extension (emitted verbatim), so the long tail of
// foreign EAS codes is not enumerated here.
var EndpointSchemes = map[string]string{
	"0088": SchemeGLN,
	"0184": SchemeDKCVR,
	"0198": SchemeDKSE,
}
