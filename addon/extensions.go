package addon

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/pkg/here"
)

const (
	// ChargeKeyExcise marks a bill charge as a Danish excise duty; its SKAT
	// code travels in the ExtKeyDutyCode extension.
	ChargeKeyExcise cbc.Key = "excise"

	// ExtKeyDutyCode identifies the SKAT excise duty code carried by a charge
	// keyed with ChargeKeyExcise.
	ExtKeyDutyCode cbc.Key = "dk-oioubl-duty-code"

	// ExtKeyResponseCode carries the OIOUBL response code on a status line,
	// preserving the wire value across GOBL's four status keys.
	ExtKeyResponseCode cbc.Key = "dk-oioubl-response-code"
)

// The six ExtKeyResponseCode values, from OIOUBL's responsecode-1.1 list.
const (
	ResponseCodeBusinessAccept  cbc.Code = "BusinessAccept"
	ResponseCodeBusinessReject  cbc.Code = "BusinessReject"
	ResponseCodeProfileAccept   cbc.Code = "ProfileAccept"
	ResponseCodeProfileReject   cbc.Code = "ProfileReject"
	ResponseCodeTechnicalAccept cbc.Code = "TechnicalAccept"
	ResponseCodeTechnicalReject cbc.Code = "TechnicalReject"
)

var extensions = []*cbc.Definition{
	{
		Key: ExtKeyDutyCode,
		Name: i18n.String{
			i18n.EN: "SKAT excise duty code",
			i18n.DA: "Punktafgiftskode",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Code identifying a Danish excise duty ("punktafgift") from the SKAT
				tax scheme codelist (OIOUBL ~urn:oioubl:id:taxschemeid~), set on a
				document or line charge whose key is ~excise~. OIOUBL emits the
				charge as its own ~cac:TaxTotal~ with this code as the
				~cac:TaxScheme/cbc:ID~.

				For example, ~16~ is the duty on chocolate and confectionery
				("Chokolade- og sukkervarerafgift") and ~66~ the vehicle
				registration tax ("Registreringsafgift af motorkøretøjer").

				The value set is deliberately left open: the codelist has both
				gained and lost codes across taxschemeid versions, and includes
				non-numeric codes (e.g. ~21d~).
			`),
		},
	},
	{
		Key: ExtKeyResponseCode,
		Name: i18n.String{
			i18n.EN: "OIOUBL Response Code",
			i18n.DA: "OIOUBL Svarkode",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				The response code from OIOUBL's ~responsecode-1.1~ list, set on an
				ApplicationResponse status line. OIOUBL distinguishes six answers where
				GOBL's status keys have four, so the wire value is kept here: the key
				carries the meaning, this extension the exact code. Absent on outgoing
				documents, the key decides the code.
			`),
		},
		Values: []*cbc.Definition{
			{
				Code: ResponseCodeBusinessAccept,
				Name: i18n.String{i18n.EN: "Business Accept"},
				Desc: i18n.String{i18n.EN: "The document is accepted."},
			},
			{
				Code: ResponseCodeBusinessReject,
				Name: i18n.String{i18n.EN: "Business Reject"},
				Desc: i18n.String{i18n.EN: "The document is refused."},
			},
			{
				Code: ResponseCodeProfileAccept,
				Name: i18n.String{i18n.EN: "Profile Accept"},
				Desc: i18n.String{i18n.EN: "The document's business profile is supported."},
			},
			{
				Code: ResponseCodeProfileReject,
				Name: i18n.String{i18n.EN: "Profile Reject"},
				Desc: i18n.String{i18n.EN: "The document's business profile is not supported."},
			},
			{
				Code: ResponseCodeTechnicalAccept,
				Name: i18n.String{i18n.EN: "Technical Accept"},
				Desc: i18n.String{i18n.EN: "The document was received and can be read."},
			},
			{
				Code: ResponseCodeTechnicalReject,
				Name: i18n.String{i18n.EN: "Technical Reject"},
				Desc: i18n.String{i18n.EN: "The document could not be read or processed."},
			},
		},
	},
}
