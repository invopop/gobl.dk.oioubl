package addon

import (
	"strings"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/iso"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/tax"
)

// oioublEndpointScheme is the OIOUBL endpoint-identifier scheme URI
// (urn:oioubl:scheme:endpointid-1.1, the codelist's declared "Identification
// Scheme"), the OIOUBL counterpart to Peppol's iso6523-actorid-upis. Participants
// are carried as org.Endpoint URIs of the form
// "urn:oioubl:scheme:endpointid-1.1::<scheme>:<value>" (e.g. DK:CVR:12345674),
// where <scheme> is one of the nine symbolic codelist values (DK:CVR, DK:SE, GLN,
// "ISO 6523" for OVT, …).
const oioublEndpointScheme = "urn:oioubl:scheme:endpointid-1.1"

// OIOUBLEndpointURI builds the OIOUBL participant endpoint URI for a symbolic
// scheme and participant code. The value is colon-free (CVR/SE/GLN/IBAN/CPR/…),
// so the scheme — which may itself contain a colon (DK:CVR) — is recovered on the
// last colon when reading.
func OIOUBLEndpointURI(scheme, code string) string {
	return oioublEndpointScheme + "::" + scheme + ":" + code
}

// ParseOIOUBLEndpoint splits an OIOUBL endpoint URI into its symbolic scheme and
// participant code, returning ok=false for any other URI. The participant code is
// colon-free, so the scheme is recovered up to the last colon.
func ParseOIOUBLEndpoint(uri string) (scheme, code string, ok bool) {
	rest, found := strings.CutPrefix(uri, oioublEndpointScheme+"::")
	if !found {
		return "", "", false
	}
	i := strings.LastIndex(rest, ":")
	if i <= 0 || i == len(rest)-1 {
		return "", "", false
	}
	return rest[:i], rest[i+1:], true
}

// normalizeParty resolves a party's NemHandel participant to an org.Endpoint
// under the OIOUBL endpoint-identifier scheme — the going-forward routing field,
// since org.Inbox is deprecated. It (1) migrates a scheme/code inbox to the
// equivalent endpoint, and (2) for a Danish party still lacking one, derives the
// CVR participant (DK:CVR) from the tax identity. An explicit DK:SE, GLN or
// foreign participant supplied by the producer is preserved.
func normalizeParty(p *org.Party) {
	if len(p.Endpoints) == 0 {
		migrateInboxesToEndpoints(p)
	}
	if p.TaxID == nil || p.TaxID.Country != "DK" || p.TaxID.Code == cbc.CodeEmpty {
		return
	}
	if len(p.Endpoints) == 0 {
		p.Endpoints = append(p.Endpoints, &org.Endpoint{
			URI: cbc.URI(OIOUBLEndpointURI(SchemeDKCVR, p.TaxID.Code.String())),
		})
	}
	// Legal identity: OIOUBL's PartyLegalEntity/CompanyID is the CVR. Set it
	// explicitly so the converter maps it rather than fabricating one from the tax
	// ID. Left untouched if a legal identity already exists.
	if !hasLegalIdentity(p) {
		p.Identities = append(p.Identities, &org.Identity{
			Scope: org.IdentityScopeLegal,
			Code:  p.TaxID.Code,
			Ext:   tax.ExtensionsOf(cbc.CodeMap{iso.ExtKeySchemeID: cbc.Code(icd0184)}),
		})
	}
}

// migrateInboxesToEndpoints converts each scheme/code org.Inbox into the
// equivalent OIOUBL org.Endpoint and drops it, since org.Inbox is deprecated in
// favour of org.Endpoint. A numeric ISO 6523 ICD inbox scheme is mapped to its
// symbolic OIOUBL scheme (0184→DK:CVR); email/URL inboxes carry no scheme/code
// participant and are left untouched.
func migrateInboxesToEndpoints(p *org.Party) {
	kept := p.Inboxes[:0]
	for _, in := range p.Inboxes {
		if in == nil {
			continue
		}
		if in.Scheme == cbc.CodeEmpty || in.Code == cbc.CodeEmpty {
			kept = append(kept, in)
			continue
		}
		scheme := in.Scheme.String()
		if s := SchemeForICD(scheme); s != "" {
			scheme = s.String()
		}
		p.Endpoints = append(p.Endpoints, &org.Endpoint{
			Label: in.Label,
			URI:   cbc.URI(OIOUBLEndpointURI(scheme, in.Code.String())),
		})
	}
	p.Inboxes = kept
}

// icd0184 is the ISO 6523 ICD for the Danish CVR register, carried on the legal
// identity for the generic (non-OIOUBL) serializer; the OIOUBL converter emits
// the symbolic DK:CVR scheme instead.
const icd0184 = "0184"

// hasLegalIdentity reports whether the party already carries a legal-scope identity.
func hasLegalIdentity(p *org.Party) bool {
	for _, id := range p.Identities {
		if id != nil && id.Scope == org.IdentityScopeLegal {
			return true
		}
	}
	return false
}

// normalizePayInstructions prepares an invoice payment instruction for OIOUBL: it
// rewrites the EN 16931 credit-transfer means to OIOUBL's code (see
// oioublPaymentMeans) and records the paymentchannelcode-1.1 value in the
// dk-oioubl-payment-channel extension, so the gobl.ubl serializer emits the means
// code and cbc:PaymentChannelCode directly.
func normalizePayInstructions(instr *pay.Instructions) {
	instr.Ext = oioublPaymentMeans(instr.Ext)
	ch := oioublPaymentChannel(instr.Ext.Get(untdid.ExtKeyPaymentMeans))
	if ch == "" {
		// Clear any channel left by a previous means: a stale DK:GIRO/DK:FIK on a
		// channel-less means is wire-fatal (e.g. F-LIB321).
		instr.Ext = instr.Ext.Delete(ExtKeyPaymentChannel)
		return
	}
	instr.Ext = instr.Ext.Set(ExtKeyPaymentChannel, ch)
}

// oioublPaymentMeans rewrites the EN 16931 credit-transfer means (UNTDID 4461 code
// 30) to OIOUBL's bank-transfer code 31, which OIOUBL's PaymentMeansCode codelist
// requires in its place (F-LIB100). Other means pass through unchanged.
func oioublPaymentMeans(ext tax.Extensions) tax.Extensions {
	if ext.Get(untdid.ExtKeyPaymentMeans) == "30" {
		return ext.Set(untdid.ExtKeyPaymentMeans, "31")
	}
	return ext
}

// oioublPaymentChannel maps a UNTDID 4461 payment means to its OIOUBL payment
// channel: Giro (50) → DK:GIRO, FIK (93) → DK:FIK, and the account-transfer
// means (30/31 bank transfers, 58 SEPA credit transfer) → IBAN. Every other
// accepted means (cash, cheque, direct debit, cards, clearing) settles outside
// a payment channel and carries none. (42 is not an accepted means — see
// validPaymentMeansCodes.)
func oioublPaymentChannel(means cbc.Code) cbc.Code {
	switch means {
	case "50":
		return ExtValuePaymentChannelGiro
	case "93":
		return ExtValuePaymentChannelFIK
	case "30", "31", "58":
		return ExtValuePaymentChannelIBAN
	default:
		return ""
	}
}

// normalizeTaxCombo strips the EN 16931 UNTDID tax-category extension from VAT
// combos. The gobl.ubl serializer derives the OIOUBL taxcategoryid-1.1 code from
// the GOBL VAT key, so the UNTDID code is redundant and only adds confusing noise
// to an OIOUBL document. en16931 normalizes first (it is required), setting the
// key, so removing the extension here is lossless.
func normalizeTaxCombo(c *tax.Combo) {
	if c.Category == tax.CategoryVAT {
		c.Ext = c.Ext.Delete(untdid.ExtKeyTaxCategory)
	}
}

// normalizeTaxNote strips the same UNTDID tax-category extension from a VAT tax
// note; the note's key identifies the rate it applies to.
func normalizeTaxNote(n *tax.Note) {
	if n.Category == tax.CategoryVAT {
		n.Ext = n.Ext.Delete(untdid.ExtKeyTaxCategory)
	}
}

// taxCategoryMapsToOIOUBL reports whether a GOBL VAT key has an OIOUBL
// taxcategoryid-1.1 equivalent. The gobl.ubl serializer maps the key to the
// OIOUBL code directly (standard → StandardRated, zero/exempt → ZeroRated as
// OIOUBL 2.1 has no exempt category, reverse-charge → ReverseCharge); this gates
// the addon's own document-type and category rules. Export, intra-community and
// outside-scope have no OIOUBL category.
func taxCategoryMapsToOIOUBL(key cbc.Key) bool {
	switch key {
	case tax.KeyStandard, tax.KeyZero, tax.KeyExempt, tax.KeyReverseCharge, "":
		return true
	}
	return false
}

// normalizeStatusLine records the OIOUBL responsecode-1.1 value in the
// dk-oioubl-response-code extension, derived from the GOBL status event, so the
// gobl.ubl serializer emits cac:Response/cbc:ResponseCode directly. On an inbound
// document the line carries the parsed extension but no event, so the mapping is
// applied in reverse to recover the GOBL status event.
//
// An extension that still reverse-maps to the current key is left untouched, so
// an inbound ProfileReject (which folds into the error event alongside
// TechnicalReject) survives recalculation; an extension that no longer matches
// the key — a stale value after the key was edited — is overwritten.
func normalizeStatusLine(line *bill.StatusLine) {
	if code := oioublResponseCode(line.Key); code != "" {
		if cur := line.Ext.Get(ExtKeyResponseCode); cur == "" || goblStatusEvent(cur) != line.Key {
			line.Ext = line.Ext.Set(ExtKeyResponseCode, code)
		}
	}
	if line.Key == "" {
		if event := goblStatusEvent(line.Ext.Get(ExtKeyResponseCode)); event != "" {
			line.Key = event
		}
	}
}

// oioublResponseCode maps a GOBL status event to its OIOUBL responsecode-1.1
// value. Events without an OIOUBL counterpart (issued, processing, paid, …) map
// to nothing and are rejected by the addon validation rules (F-APR018).
func oioublResponseCode(event cbc.Key) cbc.Code {
	switch event {
	case bill.StatusLineAccepted:
		return ExtValueResponseCodeBusinessAccept
	case bill.StatusLineRejected:
		return ExtValueResponseCodeBusinessReject
	case bill.StatusLineAcknowledged:
		return ExtValueResponseCodeTechnicalAccept
	case bill.StatusLineError:
		return ExtValueResponseCodeTechnicalReject
	}
	return ""
}

// goblStatusEvent reverses oioublResponseCode for inbound documents. ProfileReject
// has no dedicated GOBL event and folds into error, alongside TechnicalReject.
func goblStatusEvent(code cbc.Code) cbc.Key {
	switch code {
	case ExtValueResponseCodeBusinessAccept:
		return bill.StatusLineAccepted
	case ExtValueResponseCodeBusinessReject:
		return bill.StatusLineRejected
	case ExtValueResponseCodeTechnicalAccept:
		return bill.StatusLineAcknowledged
	case ExtValueResponseCodeTechnicalReject, ExtValueResponseCodeProfileReject:
		return bill.StatusLineError
	}
	return ""
}
