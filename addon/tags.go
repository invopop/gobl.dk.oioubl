package addon

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/pkg/here"
	"github.com/invopop/gobl/tax"
)

// TagAdvis marks an OIOUBL Reminder (a bill.Payment of type "request") as an
// advisory notice rather than a formal dunning reminder. It maps to the
// remindertypecode-1.1 value "Advis" on cbc:ReminderTypeCode (F-REM061); an
// untagged reminder defaults to "Reminder" (formal dunning).
const TagAdvis cbc.Key = "advis"

// paymentTags declares the OIOUBL document-variant tags for bill.Payment.
var paymentTags = &tax.TagSet{
	Schema: bill.ShortSchemaPayment,
	List: []*cbc.Definition{
		{
			Key:  TagAdvis,
			Name: i18n.String{i18n.EN: "Advisory notice", i18n.DA: "Advis"},
			Desc: i18n.String{
				i18n.EN: here.Doc(`
					Marks an OIOUBL Reminder as an advisory notice (such as an account
					statement) instead of a formal dunning reminder. Emitted as the
					remindertypecode-1.1 value "Advis"; an untagged reminder is "Reminder".
				`),
			},
		},
	},
}
