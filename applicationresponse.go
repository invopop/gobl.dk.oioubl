package dkoioubl

import (
	"encoding/xml"
	"strconv"

	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
)

// NamespaceUBLApplicationResponse is the UBL 2.1 ApplicationResponse root namespace.
const NamespaceUBLApplicationResponse = "urn:oasis:names:specification:ubl:schema:xsd:ApplicationResponse-2"

// ApplicationResponse represents a UBL 2.1 ApplicationResponse document, used to
// return a response (accept or reject) for a previously received document.
type ApplicationResponse struct {
	XMLName      xml.Name
	CACNamespace string `xml:"xmlns:cac,attr"`
	CBCNamespace string `xml:"xmlns:cbc,attr"`
	UBLNamespace string `xml:"xmlns,attr"`

	UBLVersionID    string  `xml:"cbc:UBLVersionID,omitempty"`
	CustomizationID string  `xml:"cbc:CustomizationID,omitempty"`
	ProfileID       *IDType `xml:"cbc:ProfileID,omitempty"`
	ID              string  `xml:"cbc:ID"`
	UUID            string  `xml:"cbc:UUID,omitempty"`
	IssueDate       string  `xml:"cbc:IssueDate"`
	IssueTime       string  `xml:"cbc:IssueTime,omitempty"`

	Note []string `xml:"cbc:Note,omitempty"`

	SenderParty      *Party              `xml:"cac:SenderParty"`
	ReceiverParty    *Party              `xml:"cac:ReceiverParty"`
	DocumentResponse []*DocumentResponse `xml:"cac:DocumentResponse"`
}

// DocumentResponse pairs one Response with the document it concerns; an
// ApplicationResponse carries one per status line.
type DocumentResponse struct {
	Response          *Response                  `xml:"cac:Response"`
	DocumentReference *ResponseDocumentReference `xml:"cac:DocumentReference"`
}

// Response carries the response code and an optional human description.
type Response struct {
	ReferenceID   string   `xml:"cbc:ReferenceID,omitempty"`
	ResponseCode  *IDType  `xml:"cbc:ResponseCode"`
	Description   []string `xml:"cbc:Description,omitempty"`
	EffectiveDate string   `xml:"cbc:EffectiveDate,omitempty"`
}

// ResponseDocumentReference identifies the document being responded to.
type ResponseDocumentReference struct {
	ID               string  `xml:"cbc:ID"`
	UUID             string  `xml:"cbc:UUID,omitempty"`
	IssueDate        string  `xml:"cbc:IssueDate,omitempty"`
	DocumentTypeCode *IDType `xml:"cbc:DocumentTypeCode"`
}

func newApplicationResponse(st *bill.Status) *ApplicationResponse {
	// SenderParty is who sends the response, ReceiverParty who receives it. The
	// supplier/customer roles flip with the status type (a response travels
	// customer->supplier, an update supplier->customer).
	sender, receiver := st.Customer, st.Supplier
	if st.Type == bill.StatusTypeUpdate {
		sender, receiver = st.Supplier, st.Customer
	}

	out := &ApplicationResponse{
		XMLName:         xml.Name{Local: "ApplicationResponse"},
		CACNamespace:    NamespaceCAC,
		CBCNamespace:    NamespaceCBC,
		UBLNamespace:    NamespaceUBLApplicationResponse,
		UBLVersionID:    Version,
		CustomizationID: CustomizationID,
		ID:              invoiceNumber(st.Series, st.Code),
		IssueDate:       formatDate(st.IssueDate),
		SenderParty:     newParty(sender),
		ReceiverParty:   newParty(receiver),
	}
	out.ProfileID = &IDType{Value: ProfileID}
	if !st.UUID.IsZero() {
		out.UUID = st.UUID.String()
	}
	if st.IssueTime != nil {
		out.IssueTime = st.IssueTime.String()
	}
	for _, n := range st.Notes {
		if n != nil && n.Text != "" {
			out.Note = append(out.Note, n.Text)
		}
	}

	applyParty(out.SenderParty)
	applyParty(out.ReceiverParty)
	applyResponseProfile(out, st)

	// One DocumentResponse per status line: its response (description, effective
	// date) plus a reference to the document being responded to.
	for _, line := range st.Lines {
		dr := &DocumentResponse{Response: &Response{}}
		if desc := responseDescription(line); desc != "" {
			dr.Response.Description = []string{desc}
		}
		if line.Date != nil {
			dr.Response.EffectiveDate = formatDate(*line.Date)
		}
		if line.Doc != nil {
			ref := &ResponseDocumentReference{
				ID: invoiceNumber(line.Doc.Series, line.Doc.Code),
			}
			if !line.Doc.UUID.IsZero() {
				ref.UUID = line.Doc.UUID.String()
			}
			if line.Doc.IssueDate != nil {
				ref.IssueDate = formatDate(*line.Doc.IssueDate)
			}
			dr.DocumentReference = ref
		}

		applyDocumentResponse(dr, line)

		out.DocumentResponse = append(out.DocumentResponse, dr)
	}

	return out
}

// responseDescription prefers the line description and falls back to the first
// reason's description.
func responseDescription(line *bill.StatusLine) string {
	if line.Description != "" {
		return line.Description
	}
	for _, r := range line.Reasons {
		if r != nil && r.Description != "" {
			return r.Description
		}
	}
	return ""
}

// profileTechnicalID is the technical-response profile the schematron couples
// with the TechnicalAccept response code (F-APR057/058). Both
// ApplicationResponse profiles only appear in the profileid-1.4 code list
// (schemeProfileV14), unlike invoices, which use 1.2 (see invoice.go).
const profileTechnicalID = "Procurement-TecRes-1.0"

// applyResponseProfile stamps the OIOUBL profileid-1.4 code-list attributes
// onto the ProfileID and, for a technical acknowledgement, swaps in the
// technical-response profile. F-APR057/F-APR058 bind the TechnicalAccept
// response code to that profile; every other response rides the billing profile.
func applyResponseProfile(out *ApplicationResponse, st *bill.Status) {
	if out.ProfileID == nil {
		return
	}
	out.ProfileID.SchemeAgencyID = ptr(agencyID)
	out.ProfileID.SchemeID = ptr(schemeProfileV14)
	if len(st.Lines) > 0 && st.Lines[0].Key == bill.StatusLineAcknowledged {
		out.ProfileID.Value = profileTechnicalID
	}
}

// OIOUBL responsecode-1.1 wire values. Derived from the status event (see
// responseCode), not carried in an extension. F-APR018 accepts five of the six
// codelist values (ProfileAccept is rejected).
const (
	responseCodeBusinessAccept  cbc.Code = "BusinessAccept"
	responseCodeBusinessReject  cbc.Code = "BusinessReject"
	responseCodeTechnicalAccept cbc.Code = "TechnicalAccept"
	responseCodeTechnicalReject cbc.Code = "TechnicalReject"
	responseCodeProfileReject   cbc.Code = "ProfileReject"
)

// responseCode maps a GOBL status event to its OIOUBL responsecode-1.1 value,
// or "" for events OIOUBL does not represent (rejected by F-APR018). The code
// is derived from the event rather than carried in an extension.
func responseCode(key cbc.Key) string {
	switch key {
	case bill.StatusLineAccepted:
		return string(responseCodeBusinessAccept)
	case bill.StatusLineRejected:
		return string(responseCodeBusinessReject)
	case bill.StatusLineAcknowledged:
		return string(responseCodeTechnicalAccept)
	case bill.StatusLineError:
		return string(responseCodeTechnicalReject)
	}
	return ""
}

// applyDocumentResponse stamps the OIOUBL 2.1 specifics onto a single
// DocumentResponse: the mandatory ReferenceID (F-APR016), the responsecode-1.1
// value with its code-list attributes, and the document-type code list.
func applyDocumentResponse(dr *DocumentResponse, line *bill.StatusLine) {
	resp := dr.Response
	resp.ReferenceID = strconv.Itoa(responseReferenceID(line.Index))

	if code := responseCode(line.Key); code != "" {
		resp.ResponseCode = &IDType{
			ListAgencyID: ptr(agencyID),
			ListID:       ptr(listResponseCode),
			Value:        code,
		}
	}

	if ref := dr.DocumentReference; ref != nil && line.Doc != nil {
		ref.DocumentTypeCode = &IDType{
			ListAgencyID: ptr(agencyID),
			ListID:       ptr(listResponseDocType),
			Value:        responseDocType(line.Doc.Type),
		}
	}
}

// responseReferenceID returns the 1-based line reference for the Response,
// falling back to 1 for an unset index (F-APR016 requires a non-empty value).
func responseReferenceID(index int) int {
	if index < 1 {
		return 1
	}
	return index
}

// responseDocType maps a referenced GOBL document type to the OIOUBL
// responsedocumenttypecode-1.1 value.
func responseDocType(t cbc.Key) string {
	if t == bill.InvoiceTypeCreditNote {
		return "CreditNote"
	}
	return "Invoice"
}
