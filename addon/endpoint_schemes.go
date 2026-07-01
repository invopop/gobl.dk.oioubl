package addon

import "github.com/invopop/gobl/cbc"

// OIOUBL symbolic EndpointID / company-ID schemes (F-LIB179): numeric ISO 6523
// scheme IDs are rejected on the NemHandel wire, so the converter emits these
// symbolic values. DKCVR / DKSE / GLN are the Danish identifiers; ZZZ is the
// OIOUBL "other" scheme, the only company-ID scheme valid for a non-Danish
// identifier (F-LIB189/F-LIB195).
const (
	SchemeDKCVR = "DK:CVR"
	SchemeDKSE  = "DK:SE"
	SchemeGLN   = "GLN"
	SchemeZZZ   = "ZZZ"
)

// endpointSchemes maps the Danish ISO 6523 ICDs to their symbolic OIOUBL
// EndpointID schemeID (F-LIB179) — the only schemes derived automatically. A
// foreign participant carries its scheme explicitly in the endpoint URI, so
// the foreign EAS codes are not listed.
// This is the single source of the codelist; the converter holds none of it and
// goes through SchemeForICD / ICDForScheme instead.
var endpointSchemes = map[string]string{
	"0088": SchemeGLN,
	"0184": SchemeDKCVR,
	"0198": SchemeDKSE,
}

// endpointICDs is the inverse of endpointSchemes (1:1, no collisions).
var endpointICDs = func() map[string]string {
	m := make(map[string]string, len(endpointSchemes))
	for icd, scheme := range endpointSchemes {
		m[scheme] = icd
	}
	return m
}()

// SchemeForICD returns the symbolic OIOUBL scheme for a Danish ISO 6523 ICD
// (0184→DK:CVR, 0198→DK:SE, 0088→GLN), or "" when the ICD has no Danish
// counterpart (a foreign participant supplies its scheme via the extension).
func SchemeForICD(icd string) cbc.Code {
	return cbc.Code(endpointSchemes[icd])
}

// ICDForScheme returns the ISO 6523 ICD for a symbolic Danish scheme. It is the
// inverse the converter uses to rebuild a canonical org.Endpoint when parsing an
// inbound document — org.Inbox is deprecated, so participants are restored as
// Endpoints, which need the numeric ICD. A foreign symbolic scheme has no Danish
// ICD and returns ok=false (the converter then restores it as an inbox).
func ICDForScheme(scheme string) (icd string, ok bool) {
	icd, ok = endpointICDs[scheme]
	return icd, ok
}
