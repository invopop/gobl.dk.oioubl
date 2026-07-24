package dkoioubl

// Moves RequestedDeliveryPeriod into EstimatedDeliveryPeriod, the field the
// generic parser actually reads (F-INV087/089/090).
func (ui *Invoice) stripDeliveryFlavor() {
	for _, d := range ui.Delivery {
		if d == nil || d.RequestedDeliveryPeriod == nil {
			continue
		}
		d.EstimatedDeliveryPeriod = d.RequestedDeliveryPeriod
		d.RequestedDeliveryPeriod = nil
	}
}
