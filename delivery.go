package oioubl

import (
	ubl "github.com/invopop/gobl.ubl"

	"github.com/invopop/gobl/bill"
)

// applyDelivery adjusts the base's already-built delivery for OIOUBL (F-INV087/089/090, F-LIB187).
func applyDelivery(d *ubl.Delivery, del *bill.DeliveryDetails) {
	if d == nil || del == nil {
		return
	}

	if del.Period != nil {
		// GOBL never sets Date alongside Period, so OIOUBL's RequestedDeliveryPeriod replaces both date fields.
		d.LatestDeliveryDate = nil
		d.ActualDeliveryDate = nil
		d.RequestedDeliveryPeriod = &ubl.Period{
			StartDate: formatDate(del.Period.Start),
			EndDate:   formatDate(del.Period.End),
		}
	}

	if d.DeliveryParty != nil {
		// OIOUBL requires a non-empty CompanyID on PartyLegalEntity (F-LIB187);
		// the delivery party never has one, so drop the element entirely.
		d.DeliveryParty.PartyLegalEntity = nil
	}

	if d.DeliveryLocation != nil && del.Receiver != nil {
		applyAddress(d.DeliveryLocation.Address, firstAddress(del.Receiver.Addresses))
	}

	applyDeliveryLocationScheme(d, del)
}

// applyDeliveryLocationScheme says which scheme the location's identifier came
// from, e.g. GLN. The base only reads the attribute; OIOUBL accepts any scheme.
func applyDeliveryLocationScheme(d *ubl.Delivery, del *bill.DeliveryDetails) {
	if d.DeliveryLocation == nil || d.DeliveryLocation.ID == nil {
		return
	}
	if d.DeliveryLocation.ID.SchemeID != nil || len(del.Identities) == 0 {
		return
	}
	if id := del.Identities[0]; id != nil && id.Label != "" {
		d.DeliveryLocation.ID.SchemeID = ptr(id.Label)
	}
}
