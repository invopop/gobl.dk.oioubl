package oioubl

// stripDelivery moves the delivery period to the field the generic parser
// actually reads (F-INV087/089/090).
func (ui *Invoice) stripDelivery() {
	for _, d := range ui.Delivery {
		if d == nil || d.RequestedDeliveryPeriod == nil {
			continue
		}
		d.EstimatedDeliveryPeriod = d.RequestedDeliveryPeriod
		d.RequestedDeliveryPeriod = nil
	}
}
