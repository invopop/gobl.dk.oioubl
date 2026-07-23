package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/org"
)

// applyDelivery adjusts the base's already-built delivery (via gobl.ubl's
// NewDelivery) for OIOUBL: uses RequestedDeliveryPeriod for the period
// (F-INV087/089/090) and drops the delivery PartyLegalEntity (F-LIB187).
func applyDelivery(d *ubl.Delivery, del *bill.DeliveryDetails) {
	if d == nil || del == nil {
		return
	}

	if del.Period != nil {
		// OIOUBL only permits RequestedDeliveryPeriod for a period (F-INV087/089/090).
		d.LatestDeliveryDate = nil
		d.ActualDeliveryDate = nil
		if del.Date != nil {
			date := ubl.FormatDate(*del.Date)
			d.ActualDeliveryDate = &date
		}
		d.RequestedDeliveryPeriod = &ubl.Period{
			StartDate: ubl.FormatDate(del.Period.Start),
			EndDate:   ubl.FormatDate(del.Period.End),
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
}
