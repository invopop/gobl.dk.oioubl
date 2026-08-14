package oioubl

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"slices"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl.dk.oioubl/addon"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/uuid"
)

// ApplicationResponse is an OIOUBL 2.1 ApplicationResponse: the message
// answering a document, mapped to and from GOBL's bill.Status. A defined type
// over gobl.ubl's, not an alias, so OIOUBL's own methods attach to it without
// inheriting the generic Convert.
type ApplicationResponse ubl.ApplicationResponse

// ParseApplicationResponse is Parse for callers that already know it is a response.
func ParseApplicationResponse(data []byte) (*ApplicationResponse, error) {
	doc, err := Parse(data)
	if err != nil {
		return nil, err
	}
	ar, ok := doc.(*ApplicationResponse)
	if !ok {
		return nil, fmt.Errorf("expected application response, got %T", doc)
	}
	return ar, nil
}

// Convert runs the generic parse, then adds what it deliberately leaves
// context-specific: the status key read from OIOUBL's response code, with the
// wire value kept in the dk-oioubl-response-code extension.
func (ar *ApplicationResponse) Convert() (*gobl.Envelope, error) {
	if !slices.Contains(supportedCustomizationIDs, ar.CustomizationID) {
		return nil, fmt.Errorf("%w %q, expected one of %v", ErrUnsupportedCustomizationID, ar.CustomizationID, supportedCustomizationIDs)
	}

	// Stripping rewrites the document, so work on a copy: the caller keeps a
	// usable document and a second Convert does not see a half-stripped one.
	work, err := ar.clone()
	if err != nil {
		return nil, err
	}
	stripParty(work.SenderParty)
	stripParty(work.ReceiverParty)

	env, err := (*ubl.ApplicationResponse)(work).Convert()
	if err != nil {
		return nil, err
	}
	st, ok := env.Extract().(*bill.Status)
	if !ok {
		return nil, ErrUnsupportedDocumentType
	}

	if err := work.addGOBLDetails(st); err != nil {
		return nil, err
	}

	st.SetAddons(append(st.GetAddons(), addon.V2)...)
	if err := env.Calculate(); err != nil {
		return nil, err
	}
	if err := env.Validate(); err != nil {
		return nil, err
	}
	return env, nil
}

// addGOBLDetails fills in the GOBL fields the generic parse leaves empty,
// reading them off the original OIOUBL document.
func (ar *ApplicationResponse) addGOBLDetails(st *bill.Status) error {
	// The generic parser never reads cbc:UUID; without this GOBL invents a
	// fresh one on every Calculate.
	if u, err := uuid.Parse(ar.UUID); err == nil {
		st.UUID = u
	}

	// The receiver answers as the supplier; a GLN-only receiver has no tax ID
	// for the regime to derive from.
	if st.Supplier == nil || st.Supplier.TaxID == nil || st.Supplier.TaxID.Country == "" {
		st.SetRegime("DK")
	}

	// The sender is the customer, the receiver the supplier.
	addPartyContact(st.Customer, ar.SenderParty)
	addPartyContact(st.Supplier, ar.ReceiverParty)

	for i, line := range st.Lines {
		if i >= len(ar.DocumentResponse) {
			break
		}
		dr := ar.DocumentResponse[i]
		// OIOUBL requires the code (F-APR015); without it the line means nothing.
		if dr == nil || dr.Response == nil || dr.Response.ResponseCode == nil || dr.Response.ResponseCode.Value == "" {
			return fmt.Errorf("document response %d carries no response code", i+1)
		}
		code := cbc.Code(dr.Response.ResponseCode.Value)
		key := addon.StatusKeyForResponseCode(code)
		if key == "" {
			return fmt.Errorf("unknown OIOUBL response code %q", code)
		}
		line.Key = key
		line.Ext = line.Ext.Set(addon.ExtKeyResponseCode, code)
	}
	return nil
}

// clone deep-copies the document so stripping cannot reach the caller's own.
func (ar *ApplicationResponse) clone() (*ApplicationResponse, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode((*ubl.ApplicationResponse)(ar)); err != nil {
		return nil, err
	}
	out := new(ubl.ApplicationResponse)
	if err := gob.NewDecoder(&buf).Decode(out); err != nil {
		return nil, err
	}
	return (*ApplicationResponse)(out), nil
}
