package addon

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/pkg/here"
)

// Extension keys for OIOUBL 2.1.
const (
	// ExtKeyPaymentID carries the OIOUBL cbc:PaymentID "kortart" code used with
	// the Giro (PaymentMeansCode 50) and FIK (PaymentMeansCode 93) payment
	// methods. The payment reference itself is carried separately in the GOBL
	// payment instruction's Ref (emitted as cbc:InstructionID).
	ExtKeyPaymentID cbc.Key = "dk-oioubl-payment-id"

	// ExtKeyReminderSequence carries the OIOUBL cbc:ReminderSequenceNumeric, the
	// 1-based position of this reminder within the dunning sequence (F-REM007).
	// GOBL has no native field for it, so it is declared on the payment.
	ExtKeyReminderSequence cbc.Key = "dk-oioubl-reminder-sequence"

	// ExtKeyAddressFormat carries the OIOUBL addressformatcode-1.1 value emitted
	// as cbc:AddressFormatCode on a party's PostalAddress (F-LIB025/027). GOBL has
	// no native address-format field, so it is declared on the party that owns the
	// address; absent, the gobl.ubl serializer defaults to StructuredLax (the
	// universally valid format that imposes no mandatory sub-fields).
	ExtKeyAddressFormat cbc.Key = "dk-oioubl-address-format"
)

// OIOUBL addressformatcode-1.1 values (F-LIB027).
const (
	// ExtValueAddressFormatStructuredDK is the fully structured Danish address:
	// PostalZone plus StreetName-or-Postbox and BuildingNumber-or-Postbox
	// (F-LIB033/034/035), no AddressLine.
	ExtValueAddressFormatStructuredDK cbc.Code = "StructuredDK"
	// ExtValueAddressFormatStructuredLax is the lenient structured address with no
	// mandatory sub-fields; the default when none is declared.
	ExtValueAddressFormatStructuredLax cbc.Code = "StructuredLax"
	// ExtValueAddressFormatStructuredID is an address reduced to a single register
	// identifier (a GLN, carried on org.Address.Number); no postal fields.
	ExtValueAddressFormatStructuredID cbc.Code = "StructuredID"
	// ExtValueAddressFormatStructuredRegion is a regional address carrying only
	// Region, district and/or Country.
	ExtValueAddressFormatStructuredRegion cbc.Code = "StructuredRegion"
	// ExtValueAddressFormatUnstructured carries the address as free-text
	// AddressLine elements only.
	ExtValueAddressFormatUnstructured cbc.Code = "Unstructured"
)

// OIOUBL paymentchannelcode-1.1 values.
const (
	ExtValuePaymentChannelIBAN cbc.Code = "IBAN"
	ExtValuePaymentChannelGiro cbc.Code = "DK:GIRO"
	ExtValuePaymentChannelFIK  cbc.Code = "DK:FIK"
)

// OIOUBL responsecode-1.1 values accepted by the ApplicationResponse schematron
// (F-APR018 allows five of the six codelist values; ProfileAccept is rejected).
const (
	ExtValueResponseCodeBusinessAccept  cbc.Code = "BusinessAccept"
	ExtValueResponseCodeBusinessReject  cbc.Code = "BusinessReject"
	ExtValueResponseCodeTechnicalAccept cbc.Code = "TechnicalAccept"
	ExtValueResponseCodeTechnicalReject cbc.Code = "TechnicalReject"
	ExtValueResponseCodeProfileReject   cbc.Code = "ProfileReject"
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
