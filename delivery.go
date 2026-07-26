package oioubl

import (
	ubl "github.com/invopop/gobl.ubl"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/org"
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
		var a *org.Address
		if len(del.Receiver.Addresses) > 0 {
			a = del.Receiver.Addresses[0]
		}
		applyAddress(d.DeliveryLocation.Address, a)
	}

	applyDeliveryLocationScheme(d, del)
}

// applyDeliveryLocationScheme names the scheme a delivery location's identifier
// belongs to, such as GLN. The base reads it when parsing but does not write it
// back, and OIOUBL puts no restriction on which scheme is named here.
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
