package oioubl

import (
	"fmt"
	"slices"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl.dk.oioubl/addon"
	ubl "github.com/invopop/gobl.ubl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/tax"
	"github.com/invopop/gobl/uuid"
)

// ApplicationResponse is an OIOUBL 2.1 ApplicationResponse: the message
// answering a document, mapped to and from GOBL's bill.Status.
type ApplicationResponse ubl.ApplicationResponse

// Convert runs the generic parse, then adds what it deliberately leaves
// context-specific: the status key read from OIOUBL's response code, with the
// wire value kept in the dk-oioubl-response-code extension.
func (ar *ApplicationResponse) Convert() (*gobl.Envelope, error) {
	if !slices.Contains(supportedCustomizationIDs, ar.CustomizationID) {
		return nil, fmt.Errorf("unsupported customization id %q, expected one of %v", ar.CustomizationID, supportedCustomizationIDs)
	}

	env, err := (*ubl.ApplicationResponse)(ar).Convert()
	if err != nil {
		return nil, err
	}
	st, ok := env.Extract().(*bill.Status)
	if !ok {
		return nil, ErrUnsupportedDocumentType
	}

	if err := ar.addGOBLDetails(st); err != nil {
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

	for i, line := range st.Lines {
		if i >= len(ar.DocumentResponse) {
			break
		}
		dr := ar.DocumentResponse[i]
		if dr == nil || dr.Response == nil || dr.Response.ResponseCode == nil {
			continue
		}
		code := dr.Response.ResponseCode.Value
		key, ok := keyForResponseCode[code]
		if !ok {
			return fmt.Errorf("unknown OIOUBL response code %q", code)
		}
		line.Key = key
		line.Ext = tax.ExtensionsOf(cbc.CodeMap{addon.ExtKeyResponseCode: cbc.Code(code)})
	}
	return nil
}
