package addon

// OIOUBL symbolic scheme identifiers emitted on the NemHandel wire (numeric ISO
// 6523 scheme IDs are rejected, so producers supply these symbolic forms). They
// belong to three distinct OIOUBL codelists and are not interchangeable across
// contexts:
//   - EndpointID/@schemeID (F-LIB179): DK:CVR, DK:SE, GLN (among others; not ZZZ).
//   - PartyLegalEntity/CompanyID/@schemeID (F-LIB189): DK:CVR, DK:CPR, ZZZ.
//   - PartyTaxScheme/CompanyID/@schemeID (F-LIB195): DK:SE, ZZZ.
// DK:CVR / DK:SE are the Danish CVR / SE numbers, GLN is the GS1 location number,
// and ZZZ is the OIOUBL "other" company-ID scheme for a non-Danish identifier.
const (
	SchemeDKCVR = "DK:CVR"
	SchemeDKSE  = "DK:SE"
	SchemeGLN   = "GLN"
	SchemeZZZ   = "ZZZ"
)
