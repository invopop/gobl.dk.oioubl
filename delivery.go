package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"

	"github.com/invopop/gobl/bill"
)

// OIOUBL: uses RequestedDeliveryPeriod for the period (F-INV087/089/090) and drops the delivery PartyLegalEntity (F-LIB187).
func newDelivery(del *bill.DeliveryDetails) *Delivery {
	if del == nil {
		return nil
	}

	out := new(Delivery)

	if del.Date != nil {
		date := ubl.FormatDate(*del.Date)
		out.ActualDeliveryDate = &date
	}

	if del.Period != nil {
		// RequestedDeliveryPeriod is the only delivery period OIOUBL permits
		// (F-INV087/089/090 forbid LatestDeliveryDate and Promised/Estimated).
		out.RequestedDeliveryPeriod = &Period{
			StartDate: ubl.FormatDate(del.Period.Start),
			EndDate:   ubl.FormatDate(del.Period.End),
		}
	}

	if del.Receiver != nil {
		out.DeliveryParty = newDeliveryParty(del.Receiver)
		// Drop PartyLegalEntity: a delivery party has no company id, and OIOUBL
		// requires a non-empty CompanyID wherever PartyLegalEntity is present (F-LIB187).
		if out.DeliveryParty != nil {
			out.DeliveryParty.PartyLegalEntity = nil
		}
		out.DeliveryLocation =
			&Location{
				Address: newAddress(del.Receiver.Addresses),
			}
		if len(del.Identities) > 0 {
			out.DeliveryLocation.ID = &IDType{Value: del.Identities[0].Code.String()}
		}
	}

	return out
}
