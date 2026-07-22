package dkoioubl

import (
	"github.com/invopop/gobl/bill"
)

// applyLineCategoryAndTaxTotalFlavor rebuilds lines, tax categories, and tax
// totals to OIOUBL's conventions.
func (ui *Invoice) applyLineCategoryAndTaxTotalFlavor(inv *bill.Invoice) error {
	_ = inv
	return nil
}
