package addon

import (
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/i18n"
	"github.com/invopop/gobl/pkg/here"
)

// ExtKeyTaxCategory carries OIOUBL's taxcategoryid-1.1 value, derived from the
// GOBL VAT key during normalization so the converter reads it instead of
// re-deriving it.
const ExtKeyTaxCategory cbc.Key = "dk-oioubl-tax-category"

// OIOUBL taxcategoryid-1.1 category codes this addon derives; OIOUBL 2.1 has
// no exempt category, so an exempt VAT combo carries ZeroRated.
const (
	TaxCategoryStandardRated cbc.Code = "StandardRated"
	TaxCategoryZeroRated     cbc.Code = "ZeroRated"
	TaxCategoryReverseCharge cbc.Code = "ReverseCharge"
)

var extensions = []*cbc.Definition{
	{
		Key: ExtKeyTaxCategory,
		Name: i18n.String{
			i18n.EN: "OIOUBL Tax Category",
			i18n.DA: "OIOUBL Afgiftskategori",
		},
		Values: []*cbc.Definition{
			{Code: TaxCategoryStandardRated, Name: i18n.String{i18n.EN: "Standard rated"}},
			{Code: TaxCategoryZeroRated, Name: i18n.String{i18n.EN: "Zero rated"}},
			{Code: TaxCategoryReverseCharge, Name: i18n.String{i18n.EN: "Reverse charge"}},
		},
		Desc: i18n.String{
			i18n.EN: here.Doc(`
				The ~dk-oioubl-tax-category~ extension carries the OIOUBL
				taxcategoryid-1.1 value for a VAT tax combo, so the converter
				can read it directly instead of re-deriving it from the GOBL
				tax key. Set automatically during normalization from the
				combo's ~key~:

				| GOBL Tax Key         | OIOUBL Category |
				| -------------------- | ---------------- |
				| ~standard~ (or none) | ~StandardRated~   |
				| ~zero~ / ~exempt~    | ~ZeroRated~       |
				| ~reverse-charge~     | ~ReverseCharge~   |

				OIOUBL 2.1 has no distinct exempt category (F-LIB309), so an
				exempt combo maps to ~ZeroRated~ like a zero-rated one.
			`),
		},
	},
}
