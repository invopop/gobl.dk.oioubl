package oioubl

import (
	ubl "github.com/invopop/gobl.ubl"
)

// stripDelivery moves the delivery details to the fields the generic parser
// actually reads (F-INV087/089/090).
func (ui *Invoice) stripDelivery() {
	for _, d := range ui.Delivery {
		if d == nil {
			continue
		}
		if d.RequestedDeliveryPeriod != nil {
			d.EstimatedDeliveryPeriod = d.RequestedDeliveryPeriod
			d.RequestedDeliveryPeriod = nil
		}
		moveDeliveryPartyAddress(d)
	}
}

// moveDeliveryPartyAddress puts a delivery party's address where EN 16931 keeps
// the place of delivery (BG-15). OIOUBL allows it on the party, EN 16931 does
// not (UBL-CR-394), so the base never reads it and the address would be lost.
func moveDeliveryPartyAddress(d *ubl.Delivery) {
	if d.DeliveryParty == nil || d.DeliveryParty.PostalAddress == nil {
		return
	}
	addr := d.DeliveryParty.PostalAddress
	d.DeliveryParty.PostalAddress = nil
	if d.DeliveryLocation == nil {
		d.DeliveryLocation = &ubl.Location{}
	}
	// A location that states its own address is the more specific one.
	if d.DeliveryLocation.Address == nil {
		d.DeliveryLocation.Address = addr
	}
}
