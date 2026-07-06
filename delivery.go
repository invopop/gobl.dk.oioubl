package dkoioubl

import "github.com/invopop/gobl/bill"

func newDelivery(del *bill.DeliveryDetails) *Delivery {
	if del == nil {
		return nil
	}

	out := new(Delivery)

	if del.Date != nil {
		date := formatDate(*del.Date)
		out.ActualDeliveryDate = &date
	}

	if del.Period != nil {
		// A delivery window maps to RequestedDeliveryPeriod — the only delivery
		// period OIOUBL permits, since it forbids LatestDeliveryDate (F-INV087)
		// and the Promised/Estimated periods (F-INV089/F-INV090).
		out.RequestedDeliveryPeriod = &Period{
			StartDate: formatDate(del.Period.Start),
			EndDate:   formatDate(del.Period.End),
		}
	}

	if del.Receiver != nil {
		out.DeliveryParty = newDeliveryParty(del.Receiver)
		// OIOUBL requires a non-empty CompanyID whenever PartyLegalEntity is
		// present (F-LIB187), but a delivery party only identifies a location and
		// carries no company id. PartyLegalEntity isn't mandatory here, so drop it
		// and keep just the PartyName.
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
