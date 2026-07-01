package addon

import (
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

	// ExtKeyAddressFormat is the party's cbc:AddressFormatCode. It lives on the
	// party, not the address, because org.Address has no extension slot.
	ExtKeyAddressFormat cbc.Key = "dk-oioubl-address-format"
)

// OIOUBL addressformatcode-1.1 values (F-LIB027); the schematron completeness
// rule for each is enforced in addresses.go.
const (
	// StructuredDK needs PostalZone, a StreetName-or-Postbox and a
	// BuildingNumber-or-Postbox (F-LIB033/034/035).
	ExtValueAddressFormatStructuredDK cbc.Code = "StructuredDK"
	// StructuredLax has no mandatory sub-fields; the default when none is declared.
	ExtValueAddressFormatStructuredLax cbc.Code = "StructuredLax"
	// StructuredID carries only a register ID: a GLN on org.Address.Number
	// (F-LIB037/038).
	ExtValueAddressFormatStructuredID cbc.Code = "StructuredID"
	// StructuredRegion carries only Region, District and/or Country (F-LIB039/040).
	ExtValueAddressFormatStructuredRegion cbc.Code = "StructuredRegion"
	// Unstructured carries only free-text AddressLine (F-LIB031).
	ExtValueAddressFormatUnstructured cbc.Code = "Unstructured"
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

// giroPaymentIDs and fikPaymentIDs are the PaymentID values OIOUBL allows for
// each method (F-LIB147 for Giro, F-LIB152 family for FIK).
var (
	giroPaymentIDs = []cbc.Code{ExtValuePaymentIDGiro01, ExtValuePaymentIDGiro04, ExtValuePaymentIDGiro15}
	fikPaymentIDs  = []cbc.Code{ExtValuePaymentIDFIK71, ExtValuePaymentIDFIK73, ExtValuePaymentIDFIK75}
)

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
		Values: []*cbc.Definition{
			{Code: ExtValuePaymentIDGiro01, Name: i18n.String{i18n.EN: "Giro payment type 01"}},
			{Code: ExtValuePaymentIDGiro04, Name: i18n.String{i18n.EN: "Giro payment type 04"}},
			{Code: ExtValuePaymentIDGiro15, Name: i18n.String{i18n.EN: "Giro payment type 15"}},
			{Code: ExtValuePaymentIDFIK71, Name: i18n.String{i18n.EN: "FIK payment type 71"}},
			{Code: ExtValuePaymentIDFIK73, Name: i18n.String{i18n.EN: "FIK payment type 73"}},
			{Code: ExtValuePaymentIDFIK75, Name: i18n.String{i18n.EN: "FIK payment type 75"}},
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
				The OIOUBL ` + "`cbc:ReminderSequenceNumeric`" + `, the 1-based position
				of this reminder within the dunning sequence (the first reminder is 1,
				the second 2, and so on). Required by F-REM007.
			`),
		},
		Pattern: `^[0-9]+$`,
	},
	{
		Key: ExtKeyAddressFormat,
		Name: i18n.String{
			i18n.EN: "OIOUBL Address Format",
			i18n.DA: "OIOUBL Adresseformat",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				The OIOUBL ` + "`addressformatcode-1.1`" + ` value emitted as
				` + "`cbc:AddressFormatCode`" + ` on a party's postal address. GOBL has no
				native address-format field, so it is declared on the party. When absent,
				the gobl.ubl serializer emits StructuredLax, which imposes no mandatory
				sub-fields.
			`),
		},
		Values: []*cbc.Definition{
			{Code: ExtValueAddressFormatStructuredDK, Name: i18n.String{i18n.EN: "Structured Danish address"}},
			{Code: ExtValueAddressFormatStructuredLax, Name: i18n.String{i18n.EN: "Lenient structured address"}},
			{Code: ExtValueAddressFormatStructuredID, Name: i18n.String{i18n.EN: "Identifier-only address"}},
			{Code: ExtValueAddressFormatStructuredRegion, Name: i18n.String{i18n.EN: "Regional address"}},
			{Code: ExtValueAddressFormatUnstructured, Name: i18n.String{i18n.EN: "Unstructured address"}},
		},
	},
}
