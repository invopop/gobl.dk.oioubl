package addon

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/pkg/here"
)

// Extension keys for OIOUBL 2.1. Each carries an OIOUBL wire value that has no
// native GOBL field; the user-facing docs live in the definitions below.
const (
	// ExtKeyReminderSequence is the reminder's position in the dunning sequence
	// (cbc:ReminderSequenceNumeric).
	ExtKeyReminderSequence cbc.Key = "dk-oioubl-reminder-sequence"
)

var extensions = []*cbc.Definition{
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
				supplied here. Mandatory on every reminder (F-REM007);.
			`),
		},
		Pattern: `^[0-9]+$`,
	},
}
