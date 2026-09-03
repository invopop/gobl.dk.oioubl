package oioubl

import (
	"fmt"
	"strconv"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl.dk.oioubl/addon"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
)

// responseCodeForKey names the wire code emitted when a status line carries no
// dk-oioubl-response-code extension. The three keys land on the codes whose
// meaning they carry; the other two wire codes are reachable through the
// extension. Acknowledged means TechnicalAccept: the schematron refuses the
// codelist's ProfileAccept (F-APR018).
var responseCodeForKey = map[cbc.Key]cbc.Code{
	bill.StatusLineAccepted:     addon.ResponseCodeBusinessAccept,
	bill.StatusLineRejected:     addon.ResponseCodeBusinessReject,
	bill.StatusLineAcknowledged: addon.ResponseCodeTechnicalAccept,
}

// buildStatus builds the plain EN16931 ApplicationResponse, then reworks it
// into OIOUBL.
func buildStatus(env *gobl.Envelope) (*ApplicationResponse, error) {
	st, ok := env.Extract().(*bill.Status)
	if !ok {
		return nil, ErrUnsupportedDocumentType
	}
	if st.Type != bill.StatusTypeResponse {
		return nil, fmt.Errorf("%w: only response statuses map to an OIOUBL ApplicationResponse", ErrUnsupportedDocumentType)
	}
	if err := ensureStatusOIOUBLAddon(env, st); err != nil {
		return nil, err
	}
	base, err := ubl.Convert(env, ubl.WithContext(ubl.ContextEN16931))
	if err != nil {
		return nil, err
	}
	ar, ok := base.(*ubl.ApplicationResponse)
	if !ok {
		return nil, fmt.Errorf("expected application response, got %T", base)
	}
	out := (*ApplicationResponse)(ar)
	out.applyOIOUBL(st)
	return out, nil
}

// ConvertApplicationResponse is Convert for callers that already know the
// document is a response status.
func ConvertApplicationResponse(env *gobl.Envelope) (*ApplicationResponse, error) {
	doc, err := Convert(env)
	if err != nil {
		return nil, err
	}
	ar, ok := doc.(*ApplicationResponse)
	if !ok {
		return nil, fmt.Errorf("expected application response, got %T", doc)
	}
	return ar, nil
}

// ensureStatusOIOUBLAddon is ensureOIOUBLAddon for a status document.
func ensureStatusOIOUBLAddon(env *gobl.Envelope, st *bill.Status) error {
	if !addon.V2.In(st.GetAddons()...) {
		st.SetAddons(append(st.GetAddons(), addon.V2)...)
		if err := env.Calculate(); err != nil {
			return err
		}
	}
	return env.Validate()
}

// applyOIOUBL reworks the plain EN16931 response into OIOUBL 2.1, filling what
// the base deliberately leaves empty off-profile: the response code, the
// document type code, the reference id and the header identifiers.
func (ar *ApplicationResponse) applyOIOUBL(st *bill.Status) {
	ar.UBLVersionID = Version
	ar.CustomizationID = CustomizationID
	if !st.UUID.IsZero() {
		ar.UUID = st.UUID.String()
	}

	// A response travels customer->supplier, so the base put the customer in
	// SenderParty and the supplier in ReceiverParty.
	applyResponseParty(ar.SenderParty, st.Customer)
	applyResponseParty(ar.ReceiverParty, st.Supplier)

	profile := ProfileID
	for i, dr := range ar.DocumentResponse {
		if i >= len(st.Lines) || dr.Response == nil {
			continue
		}
		line := st.Lines[i]
		code := line.Ext.Get(addon.ExtKeyResponseCode)
		if code == "" {
			code = responseCodeForKey[line.Key]
		}
		dr.Response.ResponseCode = &ubl.IDType{
			ListID:       ptr(codelistResponseCode),
			ListAgencyID: ptr(agencyID),
			Value:        code.String(),
		}
		if dr.Response.ReferenceID == "" {
			// A Response requires a ReferenceID (F-APR016); its position serves.
			dr.Response.ReferenceID = strconv.Itoa(i + 1)
		}
		if dr.DocumentReference != nil {
			dr.DocumentReference.DocumentTypeCode = &ubl.IDType{
				ListID:       ptr(codelistResponseDocType),
				ListAgencyID: ptr(agencyID),
				Value:        responseDocumentType(line.Doc),
			}
		}
		// The profile follows the answer: a rejection for technical or profile
		// reasons has no business profile to name (F-APR004), and TechnicalAccept
		// belongs to the technical-response profile alone (F-APR057 / F-APR058).
		switch code {
		case addon.ResponseCodeTechnicalReject, addon.ResponseCodeProfileReject:
			profile = profileNone
		case addon.ResponseCodeTechnicalAccept:
			profile = profileTecRes
		}
	}
	ar.ProfileID = newProfileID()
	ar.ProfileID.Value = profile
	if profile == profileTecRes {
		// TecRes is not in the profileid-1.2 codelist the invoices use.
		*ar.ProfileID.SchemeID = schemeProfileV16
	}
}

// The responsedocumenttypecode-1.1 values a response of ours may reference.
const (
	docTypeInvoice    = "Invoice"
	docTypeCreditNote = "CreditNote"
)

// responseDocumentType names the referenced document on OIOUBL's
// responsedocumenttypecode-1.1 list. The addon only lets untyped, standard and
// credit-note references through, so anything not a credit note is an Invoice.
func responseDocumentType(doc *org.DocumentRef) string {
	if doc != nil && doc.Type == bill.InvoiceTypeCreditNote {
		return docTypeCreditNote
	}
	return docTypeInvoice
}

// applyResponseParty adds the details the base leaves out and corrects the
// party for OIOUBL, exactly as the invoice parties are.
func applyResponseParty(p *ubl.Party, party *org.Party) {
	if p == nil || party == nil {
		return
	}
	addPartyDetails(p, party)
	fixParty(p, nil)
}
