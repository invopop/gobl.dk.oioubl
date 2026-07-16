package addon

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/pkg/here"
)

const (
	// ChargeKeyExcise marks a bill charge as a Danish excise duty, which OIOUBL
	// carries as its own cac:TaxTotal rather than a cac:AllowanceCharge. The
	// duty's SKAT code travels in the ExtKeyDutyCode extension.
	ChargeKeyExcise cbc.Key = "excise"

	// ExtKeyDutyCode identifies the SKAT excise duty code carried by a charge
	// keyed with ChargeKeyExcise.
	ExtKeyDutyCode cbc.Key = "dk-oioubl-duty-code"
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
}
