// Package addon provides extensions and validations for the Danish OIOUBL 2.1
// standard used on the NemHandel e-invoicing network.
package addon

import (
	"github.com/invopop/gobl/addons/eu/en16931"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/norm"
	"github.com/invopop/gobl/pay"
	"github.com/invopop/gobl/pkg/here"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

const (
	Key cbc.Key = "dk-oioubl"

	V2 cbc.Key = Key + "-v2"

	// SchemeDKCVR is the OIOUBL EndpointID scheme for a Danish CVR number, used
	// when deriving a participant endpoint from a Danish tax ID.
	SchemeDKCVR cbc.Code = "DK:CVR"
)

func init() {
	tax.RegisterAddonDef(newAddon())
	rules.RegisterWithGuard(
		Key.String(),
		rules.GOBL.Add("DK-OIOUBL"),
		is.InContext(tax.AddonIn(V2)),
		billInvoiceRules(),
		billStatusRules(),
		taxComboRules(),
		billChargeRules(),
		lineChargeRules(),
		payInstructionsRules(),
		// OIOUBL accepts bare invoice payment terms (ID and amount only), so EN
		// 16931's due-dates-or-notes requirement does not apply.
		rules.For(new(pay.Terms),
			rules.Ignore("GOBL-EU-EN16931-PAY-TERMS-01"),
		),
	)
	norm.RegisterWithGuard(
		is.InContext(tax.AddonIn(V2)),
		norm.For(normalizeInvoice),
		norm.For(normalizeParty),
		norm.For(normalizeTaxCombo),
	)
}

func newAddon() *tax.AddonDef {
	return &tax.AddonDef{
		Key: V2,
		Name: i18n.String{
			i18n.EN: "Danish OIOUBL 2.1",
			i18n.DA: "Dansk OIOUBL 2.1",
		},
		Requires: []cbc.Key{
			en16931.V2017,
		},
		Extensions: extensions,
		Description: i18n.String{
			i18n.EN: here.Doc(`
				Support for the Danish OIOUBL 2.1 standard used on the NemHandel
				e-invoicing network, mandatory for business-to-government (B2G)
				transactions in Denmark since 2005.

				OIOUBL 2.1 is a national profile of UBL 2.1, maintained by
				Erhvervsstyrelsen (the Danish Business Authority). Unlike many
				European profiles it predates and does not extend EN 16931.

				This addon translates the OIOUBL Schematron rules (v1.17.2, the
				hotfix live since 2026-05-18) into GOBL
				validations. OIOUBL 2.1 is scheduled to be replaced by NemHandel
				BIS 4 starting in 2028.
			`),
			i18n.DA: here.Doc(`
				Understøttelse af den danske OIOUBL 2.1-standard, som anvendes på
				NemHandel-netværket og har været obligatorisk for offentlige
				indkøb (B2G) i Danmark siden 2005.
			`),
		},
		Sources: []*cbc.Source{
			{
				Title: i18n.String{
					i18n.EN: "OIOUBL 2.1 documentation overview",
				},
				URL: "https://nemhandel.dk/vejledning-oioubl-dokumentationsoversigt",
			},
			{
				Title: i18n.String{
					i18n.EN: "OIOUBL Schematron v1.17.2 (hotfix, live 2026-05-18)",
				},
				URL: "https://git.erst.dk/openebusiness/common/-/tree/master/released/oioubl",
			},
			{
				Title: i18n.String{
					i18n.EN: "OIOUBL Schematron rules (browsable source, referenced by rule citations)",
				},
				URL: "https://git.erst.dk/openebusiness/common/-/tree/master/resources/Schematrons/OIOUBL?ref_type=heads",
			},
		},
	}
}
