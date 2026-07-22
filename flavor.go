package dkoioubl

import (
	"github.com/invopop/gobl/bill"
)

// applyOIOUBLFlavor turns gobl.ubl's plain EN16931 base into OIOUBL 2.1, in
// three stages: party/address, then line/categories/tax_total, then schemes.
func (ui *Invoice) applyOIOUBLFlavor(inv *bill.Invoice) error {
	ui.applyPartyAndAddressFlavor(inv)
	if err := ui.applyLineCategoryAndTaxTotalFlavor(inv); err != nil {
		return err
	}
	ui.applySchemeFlavor(inv)
	return nil
}
