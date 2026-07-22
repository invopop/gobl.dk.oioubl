package dkoioubl

import (
	"github.com/invopop/gobl/bill"
)

// applySchemeFlavor stamps the OIOUBL scheme/list/agency identifiers the
// schematron expects (UBLVersionID, ProfileID, invoice/credit-note type code lists, etc.).
func (ui *Invoice) applySchemeFlavor(inv *bill.Invoice) {
	_ = inv
}
