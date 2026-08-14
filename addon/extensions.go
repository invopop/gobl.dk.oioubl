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

	// ExtKeyReminderSequence is the reminder's position in the dunning sequence
	// (cbc:ReminderSequenceNumeric).
	ExtKeyReminderSequence cbc.Key = "dk-oioubl-reminder-sequence"
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
		Key: ExtKeyReminderSequence,
		Name: i18n.String{
			i18n.EN: "OIOUBL Reminder Sequence",
			i18n.DA: "OIOUBL Rykkersekvens",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				How many times this bill has been reminded. A reminder (a bill.Payment)
				restates an unpaid invoice, and OIOUBL records its position in the dunning
				sequence: 1 for the first reminder, 2 for the second, and so on. The count
				is stateful — it depends on how many prior reminders were sent, not on
				anything in the document — so it has no native GOBL field and must be
				supplied here. Mandatory on every reminder (F-REM007).
			`),
		},
		Pattern: `^[0-9]+$`,
	},
}
