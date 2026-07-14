package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/org"
)

// OIOUBL: uses RequestedDeliveryPeriod for the period (F-INV087/089/090) and drops the delivery PartyLegalEntity (F-LIB187).
func newDelivery(del *bill.DeliveryDetails) *Delivery {
	if del == nil {
		return nil
	}

	out := ubl.NewDelivery(del, ubl.Context{})

	if del.Period != nil {
		// RequestedDeliveryPeriod is the only delivery period OIOUBL permits
		// (F-INV087/089/090 forbid LatestDeliveryDate and Promised/Estimated);
		// the base sets LatestDeliveryDate/ActualDeliveryDate for a period instead.
		out.LatestDeliveryDate = nil
		out.ActualDeliveryDate = nil
		if del.Date != nil {
			date := ubl.FormatDate(*del.Date)
			out.ActualDeliveryDate = &date
		}
		out.RequestedDeliveryPeriod = &Period{
			StartDate: ubl.FormatDate(del.Period.Start),
			EndDate:   ubl.FormatDate(del.Period.End),
		}
	}

	if out.DeliveryParty != nil {
		// Drop PartyLegalEntity: a delivery party has no company id, and OIOUBL
		// requires a non-empty CompanyID wherever PartyLegalEntity is present (F-LIB187).
		out.DeliveryParty.PartyLegalEntity = nil
	}

	if out.DeliveryLocation != nil && del.Receiver != nil {
		var a *org.Address
		if len(del.Receiver.Addresses) > 0 {
			a = del.Receiver.Addresses[0]
		}
		decorateAddress(out.DeliveryLocation.Address, a)
	}

	return out
}
