package dkoioubl

import (
	"cloud.google.com/go/civil"

	"github.com/invopop/gobl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cal"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/tax"
	"github.com/invopop/gobl/uuid"
)

// Convert turns a parsed OIOUBL ApplicationResponse into a GOBL envelope
// wrapping a bill.Status.
func (ar *ApplicationResponse) Convert() (*gobl.Envelope, error) {
	st, err := ar.goblStatus()
	if err != nil {
		return nil, err
	}

	env := gobl.NewEnvelope()
	if err := env.Insert(st); err != nil {
		return nil, err
	}
	return env, nil
}

func (ar *ApplicationResponse) goblStatus() (*bill.Status, error) {
	out := &bill.Status{
		Addons:   tax.Addons{List: Addons},
		Type:     bill.StatusTypeResponse,
		Code:     cbc.Code(ar.ID),
		Supplier: goblParty(ar.ReceiverParty),
		Customer: goblParty(ar.SenderParty),
	}

	issueDate, err := parseDate(ar.IssueDate)
	if err != nil {
		return nil, err
	}
	out.IssueDate = issueDate

	if ar.IssueTime != "" {
		ct, err := civil.ParseTime(ar.IssueTime)
		if err != nil {
			return nil, err
		}
		out.IssueTime = &cal.Time{Time: ct}
	}

	for _, n := range ar.Note {
		out.Notes = append(out.Notes, &org.Note{Text: n})
	}

	for _, dr := range ar.DocumentResponse {
		line, err := goblStatusLine(dr)
		if err != nil {
			return nil, err
		}
		out.Lines = append(out.Lines, line)
	}

	return out, nil
}

// goblStatusLine maps a single UBL DocumentResponse to a GOBL status line.
func goblStatusLine(dr *DocumentResponse) (*bill.StatusLine, error) {
	line := new(bill.StatusLine)
	if dr == nil {
		return line, nil
	}

	if r := dr.Response; r != nil {
		if len(r.Description) > 0 {
			line.Description = r.Description[0]
		}
		if r.EffectiveDate != "" {
			d, err := parseDate(r.EffectiveDate)
			if err != nil {
				return nil, err
			}
			line.Date = &d
		}
	}

	if ref := dr.DocumentReference; ref != nil {
		doc := &org.DocumentRef{
			Code: cbc.Code(ref.ID),
		}
		if ref.UUID != "" {
			doc.UUID = uuid.UUID(ref.UUID)
		}
		if ref.IssueDate != "" {
			d, err := parseDate(ref.IssueDate)
			if err != nil {
				return nil, err
			}
			doc.IssueDate = &d
		}
		line.Doc = doc
	}

	applyStatusLine(line, dr)

	return line, nil
}

// applyStatusLine recovers the GOBL status event from the parsed OIOUBL
// responsecode-1.1 value and records the document-type code on the status line.
func applyStatusLine(line *bill.StatusLine, dr *DocumentResponse) {
	if r := dr.Response; r != nil && r.ResponseCode != nil && r.ResponseCode.Value != "" {
		if event := goblStatusEvent(r.ResponseCode.Value); event != "" {
			line.Key = event
		}
	}
	if ref := dr.DocumentReference; ref != nil && line.Doc != nil &&
		ref.DocumentTypeCode != nil && ref.DocumentTypeCode.Value == "CreditNote" {
		line.Doc.Type = bill.InvoiceTypeCreditNote
	}
}

// goblStatusEvent maps an OIOUBL responsecode-1.1 value to its GOBL status event;
// ProfileReject has no dedicated GOBL event and folds into error alongside
// TechnicalReject.
func goblStatusEvent(code string) cbc.Key {
	switch cbc.Code(code) {
	case responseCodeBusinessAccept:
		return bill.StatusLineAccepted
	case responseCodeBusinessReject:
		return bill.StatusLineRejected
	case responseCodeTechnicalAccept:
		return bill.StatusLineAcknowledged
	case responseCodeTechnicalReject, responseCodeProfileReject:
		return bill.StatusLineError
	}
	return ""
}
