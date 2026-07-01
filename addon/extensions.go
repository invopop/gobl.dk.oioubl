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

	// ExtKeyTaxScheme carries the OIOUBL taxschemeid-1.1 duty-type code for a
	// non-VAT excise duty (chocolate/sugar, mineral water, packaging, …) that GOBL
	// models as a VAT-rated bill.Charge/LineCharge. Its presence routes the charge
	// to a cac:TaxTotal/TaxSubtotal with cac:TaxCategory/cbc:ID "Excise" (the
	// official ERST form) instead of a plain cac:AllowanceCharge; the gobl.ubl
	// serializer takes the scheme Name from the charge reason and derives the
	// cbc:TaxTypeCode from the amount (StandardRated when positive, ZeroRated when
	// zero). GOBL has no native field for the OIOUBL duty-type code, so it is
	// declared on the charge.
	ExtKeyTaxScheme cbc.Key = "dk-oioubl-tax-scheme"

	// ExtKeyReminderType carries the OIOUBL remindertypecode-1.1 value emitted as
	// cbc:ReminderTypeCode on a Reminder, the Danish dunning document mapped from a
	// bill.Payment of type "request". OIOUBL allows two values, Reminder and Advis
	// (F-REM006/059/060/061); GOBL has no native field, so it is declared on the
	// payment.
	ExtKeyReminderType cbc.Key = "dk-oioubl-reminder-type"

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

	// ExtKeyAddressID carries the address identifier (typically a GS1 GLN) emitted
	// as cbc:ID for a StructuredID address (F-LIB037). GOBL models no address-level
	// identifier, so it travels on the party extension alongside the format.
	ExtKeyAddressID cbc.Key = "dk-oioubl-address-id"

	// ExtKeyAddressDistrict carries the district name emitted as cbc:District for a
	// StructuredRegion address (F-LIB039). GOBL has no district field (it offers
	// region and country), so it travels on the party extension.
	ExtKeyAddressDistrict cbc.Key = "dk-oioubl-address-district"
)

// OIOUBL remindertypecode-1.1 values (F-REM061).
const (
	ExtValueReminderTypeReminder cbc.Code = "Reminder"
	ExtValueReminderTypeAdvis    cbc.Code = "Advis"
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
	// ExtValueAddressFormatStructuredID is an address reduced to a single
	// identifier (dk-oioubl-address-id, typically a GLN); no postal fields.
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
		Key: ExtKeyTaxScheme,
		Name: i18n.String{
			i18n.EN: "OIOUBL Tax Scheme (excise duty)",
			i18n.DA: "OIOUBL Afgiftstype",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				The OIOUBL ` + "`taxschemeid-1.1`" + ` duty-type code identifying a
				non-VAT excise duty (e.g. mineral-water, chocolate/sugar or packaging
				tax) that GOBL models as a VAT-rated charge. Declared on a
				` + "`bill.Charge`" + ` or ` + "`bill.LineCharge`" + `, it routes the
				charge to a ` + "`cac:TaxTotal/cac:TaxSubtotal`" + ` with
				` + "`cac:TaxCategory/cbc:ID`" + ` "Excise" instead of a plain
				` + "`cac:AllowanceCharge`" + `; the serializer takes the scheme name
				from the charge reason and derives ` + "`cbc:TaxTypeCode`" + ` from the
				amount (StandardRated when positive, ZeroRated when zero).
			`),
		},
		Pattern: `^[0-9a-z]+$`,
	},
	{
		Key: ExtKeyReminderType,
		Name: i18n.String{
			i18n.EN: "OIOUBL Reminder Type",
			i18n.DA: "OIOUBL Rykkertype",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				The OIOUBL ` + "`remindertypecode-1.1`" + ` value emitted as
				` + "`cbc:ReminderTypeCode`" + ` on a Reminder, the Danish dunning
				document mapped from a ` + "`bill.Payment`" + ` of type "request".
				OIOUBL accepts two values: ` + "`Advis`" + ` (an advisory notice, such
				as an account statement) and ` + "`Reminder`" + ` (a formal dunning
				reminder). Required by F-REM006/F-REM061.
			`),
		},
		Values: []*cbc.Definition{
			{Code: ExtValueReminderTypeReminder, Name: i18n.String{i18n.EN: "Reminder", i18n.DA: "Rykker"}},
			{Code: ExtValueReminderTypeAdvis, Name: i18n.String{i18n.EN: "Advisory notice", i18n.DA: "Advis"}},
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
	{
		Key: ExtKeyAddressID,
		Name: i18n.String{
			i18n.EN: "OIOUBL Address Identifier",
			i18n.DA: "OIOUBL Adresse-ID",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				The identifier emitted as ` + "`cbc:ID`" + ` for a StructuredID address
				(F-LIB037), typically a GS1 GLN. GOBL has no address-level identifier, so
				it is declared on the party alongside the address format.
			`),
		},
		Pattern: `^.+$`,
	},
	{
		Key: ExtKeyAddressDistrict,
		Name: i18n.String{
			i18n.EN: "OIOUBL Address District",
			i18n.DA: "OIOUBL Adressedistrikt",
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				The district name emitted as ` + "`cbc:District`" + ` for a
				StructuredRegion address (F-LIB039). GOBL offers region and country but no
				district, so it is declared on the party alongside the address format.
			`),
		},
		Pattern: `^.+$`,
	},
}
