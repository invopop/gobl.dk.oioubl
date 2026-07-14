package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
)

// OIOUBL: also parses the RequestedDeliveryPeriod (F-INV087/089/090), which
// the base doesn't read, into the delivery period.
func (ui *Invoice) goblAddDelivery(out *bill.Invoice) error {
	if err := (*ubl.Invoice)(ui).GoblAddDelivery(out); err != nil {
		return err
	}
	for _, del := range ui.Delivery {
		if del.RequestedDeliveryPeriod == nil {
			continue
		}
		p, err := ubl.GoblPeriodDates(del.RequestedDeliveryPeriod)
		if err != nil {
			return err
		}
		if out.Delivery == nil {
			out.Delivery = &bill.DeliveryDetails{}
		}
		out.Delivery.Period = p
	}
	return nil
}
