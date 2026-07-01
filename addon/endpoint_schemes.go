package addon

// OIOUBL symbolic EndpointID / company-ID schemes (F-LIB179): numeric ISO 6523
// scheme IDs are rejected on the NemHandel wire, so only these symbolic values
// are emitted. DKCVR / DKSE / GLN are the Danish identifiers; ZZZ is the OIOUBL
// "other" scheme, the only company-ID scheme valid for a non-Danish identifier
// (F-LIB189 / F-LIB195). A participant identified by an ISO 6523 scheme must
// supply it already formatted as one of these OIOUBL endpoint schemes.
const (
	SchemeDKCVR = "DK:CVR"
	SchemeDKSE  = "DK:SE"
	SchemeGLN   = "GLN"
	SchemeZZZ   = "ZZZ"
)
