package addon

import (
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/catalogues/untdid"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/rules"
	"github.com/invopop/gobl/rules/is"
	"github.com/invopop/gobl/tax"
)

// validDocumentTypes are the UNTDID 1001 codes OIOUBL accepts: 325/380/393
// (invoice) and 381 (credit note), per F-INV011 / F-CRN011.
var validDocumentTypes = []cbc.Code{"325", "380", "381", "393"}

// Rule citations reference the OIOUBL Invoice schematron (F-INV) first and the
// CreditNote equivalent (F-CRN) second.

func billInvoiceRules() *rules.Set {
	return rules.For(new(bill.Invoice),
		// BR-CO-25 does not apply: OIOUBL accepts an invoice with neither
		// payment means nor terms.
		rules.Ignore(
			"GOBL-EU-EN16931-BILL-INVOICE-06", // payment details required
			"GOBL-EU-EN16931-BILL-INVOICE-07", // payment terms required
		),
		rules.Field("tax",
			rules.Field("ext",
				rules.AssertIfPresent("01", "document type must be an OIOUBL-supported code: invoice 325/380/393 or credit note 381 (F-INV011)",
					tax.ExtensionsHasCodes(untdid.ExtKeyDocumentType, validDocumentTypes...)),
			),
			rules.Field("rounding",
				rules.AssertIfPresent("02", "amounts must be rounded per currency so the emitted lines sum to the document totals (F-INV126)",
					is.In(tax.RoundingRuleCurrency)),
			),
		),
		rules.Field("customer",
			rules.Field("endpoints",
				rules.Assert("03", "customer endpoint is required (F-INV044 / F-CRN040)",
					is.Present),
			),
		),

		rules.Field("lines",
			rules.Each(
				rules.Assert("04", "each line requires a VAT tax category (F-INV138 / F-CRN081)",
					bill.RequireLineTaxCategory(tax.CategoryVAT)),
			),
		),

		rules.Field("charges",
			rules.Each(
				rules.When(is.Expr("string(Key) != %q", ChargeKeyExcise),
					rules.Field("taxes",
						rules.Assert("05", "document-level charge requires a VAT tax for the OIOUBL TaxCategory (F-LIB226)",
							tax.SetHasCategory(tax.CategoryVAT)),
					),
				),
			),
		),
	)
}

func billChargeRules() *rules.Set {
	return rules.For(new(bill.Charge),
		rules.When(is.Expr("string(Key) == %q", ChargeKeyExcise),
			rules.Field("reason",
				rules.Assert("01", "an OIOUBL excise duty charge requires a reason for its tax-scheme name (F-LIB066)", is.Present),
			),
			rules.Field("taxes",
				rules.Assert("02", "a document-level OIOUBL excise duty requires a VAT tax stating its own VAT type (OIOUBL Skat guideline)",
					tax.SetHasCategory(tax.CategoryVAT)),
			),
			rules.Field("ext",
				rules.Assert("03", "an OIOUBL excise duty charge requires the SKAT duty code extension for its tax-scheme ID",
					tax.ExtensionsRequire(ExtKeyDutyCode)),
			),
		),
	)
}

func lineChargeRules() *rules.Set {
	return rules.For(new(bill.LineCharge),
		rules.When(is.Expr("string(Key) == %q", ChargeKeyExcise),
			rules.Field("reason",
				rules.Assert("01", "an OIOUBL excise duty charge requires a reason for its tax-scheme name (F-LIB066)", is.Present),
			),
			rules.Field("ext",
				rules.Assert("02", "an OIOUBL excise duty charge requires the SKAT duty code extension for its tax-scheme ID",
					tax.ExtensionsRequire(ExtKeyDutyCode)),
			),
		),
	)
}

// normalizeInvoice defaults to GOBL's currency rounding rule to match OIOUBL's
// own rounding, so the emitted lines sum to the document totals (F-INV126).
func normalizeInvoice(inv *bill.Invoice) {
	// The rounding rule lives on inv.Tax, which an invoice need not carry.
	if inv.Tax == nil {
		inv.Tax = new(bill.Tax)
	}
	if inv.Tax.Rounding == "" {
		inv.Tax.Rounding = tax.RoundingRuleCurrency
	}
}
