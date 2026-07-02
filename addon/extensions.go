package addon

import (
	"slices"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/pkg/here"
)

// Extension keys for OIOUBL 2.1. Each carries an OIOUBL wire value that has no
// native GOBL field; the user-facing docs live in the definitions below.
const (
	// ExtKeyPaymentID is the Giro/FIK kortart code (cbc:PaymentID); the payment
	// reference itself rides the GOBL instruction Ref (cbc:InstructionID).
	ExtKeyPaymentID cbc.Key = "dk-oioubl-payment-id"

	// ExtKeyReminderSequence is the reminder's position in the dunning sequence
	// (cbc:ReminderSequenceNumeric).
	ExtKeyReminderSequence cbc.Key = "dk-oioubl-reminder-sequence"
)

// OIOUBL Giro (code 50) PaymentID values.
const (
	ExtValuePaymentIDGiro01 cbc.Code = "01"
	ExtValuePaymentIDGiro04 cbc.Code = "04"
	ExtValuePaymentIDGiro15 cbc.Code = "15"
)

// OIOUBL FIK (code 93) PaymentID values.
const (
	ExtValuePaymentIDFIK71 cbc.Code = "71"
	ExtValuePaymentIDFIK73 cbc.Code = "73"
	ExtValuePaymentIDFIK75 cbc.Code = "75"
)

// giroPaymentIDDefs and fikPaymentIDDefs are the PaymentID codes OIOUBL allows
// for each method, named after the Nets "kortart" (payment-card type) each one
// identifies.
var (
	giroPaymentIDDefs = []*cbc.Definition{
		{Code: ExtValuePaymentIDGiro01, Name: i18n.String{i18n.EN: "Giro card, free-text message", i18n.DA: "Girokort, fri tekst"}},
		{Code: ExtValuePaymentIDGiro04, Name: i18n.String{i18n.EN: "Giro card with creditor number", i18n.DA: "Girokort med kreditornummer"}},
		{Code: ExtValuePaymentIDGiro15, Name: i18n.String{i18n.EN: "Giro card with creditor number and payment ID", i18n.DA: "Girokort med kreditornummer og betalings-id"}},
	}
	fikPaymentIDDefs = []*cbc.Definition{
		{Code: ExtValuePaymentIDFIK71, Name: i18n.String{i18n.EN: "FIK card, 15-digit payment ID", i18n.DA: "FIK-kort, 15-cifret betalings-id"}},
		{Code: ExtValuePaymentIDFIK73, Name: i18n.String{i18n.EN: "FIK card, free-text message", i18n.DA: "FIK-kort, fri tekst"}},
		{Code: ExtValuePaymentIDFIK75, Name: i18n.String{i18n.EN: "FIK card, 16-digit payment ID and message", i18n.DA: "FIK-kort, 16-cifret betalings-id og tekst"}},
	}

	// giroPaymentIDs and fikPaymentIDs are the codes alone, for the per-method
	// value check (F-LIB147 for Giro, F-LIB155 for FIK).
	giroPaymentIDs = paymentIDCodes(giroPaymentIDDefs)
	fikPaymentIDs  = paymentIDCodes(fikPaymentIDDefs)
)

// paymentIDCodes returns just the codes from a set of PaymentID definitions.
func paymentIDCodes(defs []*cbc.Definition) []cbc.Code {
	codes := make([]cbc.Code, len(defs))
	for i, d := range defs {
		codes[i] = d.Code
	}
	return codes
}

var extensions = []*cbc.Definition{
	{
		Key: ExtKeyPaymentID,
		Name: i18n.String{
			i18n.EN: "OIOUBL Payment ID (Giro/FIK kortart)",
			i18n.DA: "OIOUBL Betalings-ID (Giro/FIK kortart)",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				Identifies the OIOUBL ` + "`cbc:PaymentID`" + ` "kortart" code that
				accompanies the Danish Giro and FIK payment methods. It is mandatory
				for ` + "`PaymentMeansCode`" + ` 50 (Giro, values 01/04/15) and 93
				(FIK, values 71/73/75), per the OIOUBL Common schematron.
			`),
		},
		Values: slices.Concat(giroPaymentIDDefs, fikPaymentIDDefs),
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
				supplied here. Mandatory on every reminder (F-REM007);.
			`),
		},
		Pattern: `^[0-9]+$`,
	},
}
