package dkoioubl

import ubl "github.com/invopop/gobl.ubl"

// Invoice is the OIOUBL view of a UBL invoice. It is a defined type over
// ubl.Invoice (not an alias) so the converter can hang the OIOUBL-specific
// build and parse methods on it, and so gobl.ubl's own generic Convert method
// is not inherited (OIOUBL is never parsed as a generic UBL document). The wire
// layout is gobl.ubl's, so it marshals identically.
type Invoice ubl.Invoice

// The OIOUBL converter reuses gobl.ubl's wire model wholesale rather than
// redefining it; these aliases let the OIOUBL-specific code reference the shared
// types without qualification.
type (
	IDType                  = ubl.IDType
	Amount                  = ubl.Amount
	Quantity                = ubl.Quantity
	ExchangeRate            = ubl.ExchangeRate
	Item                    = ubl.Item
	ItemIdentification      = ubl.ItemIdentification
	CommodityClassification = ubl.CommodityClassification
	ClassifiedTaxCategory   = ubl.ClassifiedTaxCategory
	AdditionalItemProperty  = ubl.AdditionalItemProperty
	Price                   = ubl.Price
	OrderLineReference      = ubl.OrderLineReference
	InvoiceLine             = ubl.InvoiceLine
	LineDocReference        = ubl.LineDocReference
	Period                  = ubl.Period
	OrderReference          = ubl.OrderReference
	BillingReference        = ubl.BillingReference
	Reference               = ubl.Reference
	ProjectReference        = ubl.ProjectReference
	SupplierParty           = ubl.SupplierParty
	CustomerParty           = ubl.CustomerParty
	Party                   = ubl.Party
	EndpointID              = ubl.EndpointID
	Identification          = ubl.Identification
	PartyName               = ubl.PartyName
	PostalAddress           = ubl.PostalAddress
	LocationCoordinate      = ubl.LocationCoordinate
	AddressLine             = ubl.AddressLine
	Country                 = ubl.Country
	PartyTaxScheme          = ubl.PartyTaxScheme
	TaxScheme               = ubl.TaxScheme
	PartyLegalEntity        = ubl.PartyLegalEntity
	Contact                 = ubl.Contact
	TaxTotal                = ubl.TaxTotal
	TaxSubtotal             = ubl.TaxSubtotal
	TaxCategory             = ubl.TaxCategory
	MonetaryTotal           = ubl.MonetaryTotal
	AllowanceCharge         = ubl.AllowanceCharge
	PaymentMeans            = ubl.PaymentMeans
	PaymentMandate          = ubl.PaymentMandate
	CardAccount             = ubl.CardAccount
	FinancialAccount        = ubl.FinancialAccount
	Branch                  = ubl.Branch
	FinancialInstitution    = ubl.FinancialInstitution
	CreditAccount           = ubl.CreditAccount
	PaymentTerms            = ubl.PaymentTerms
	PrepaidPayment          = ubl.PrepaidPayment
	Delivery                = ubl.Delivery
	Location                = ubl.Location
	DeliveryTerms           = ubl.DeliveryTerms
	Attachment              = ubl.Attachment
	BinaryObject            = ubl.BinaryObject
	ExternalReference       = ubl.ExternalReference
	BinaryAttachment        = ubl.BinaryAttachment
)
