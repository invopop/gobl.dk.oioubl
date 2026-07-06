package dkoioubl

import (
	"encoding/base64"

	"github.com/invopop/gobl/org"
)

// AddAttachments adds an attachment to the UBL Invoice.
// This is useful for including documents like
// invoice counter values or URLs
func (ui *Invoice) AddAttachments(attachments []*org.Attachment) {
	for _, a := range attachments {
		ref := Reference{
			ID: IDType{
				Value: a.Code.String(),
			},
		}

		if a.Description != "" {
			ref.DocumentDescription = a.Description
		}

		if uuid := string(a.UUID); uuid != "" {
			ref.UUID = uuid
		}

		if a.URL != "" {
			extRef := &ExternalReference{
				URI: a.URL,
			}

			if a.MIME != "" {
				extRef.MimeCode = a.MIME
			}

			if a.Name != "" {
				extRef.FileName = a.Name
			}

			if a.Digest != nil {
				extRef.DocumentHash = a.Digest.Value
				extRef.HashAlgorithmMethod = string(a.Digest.Algorithm)
			}

			ref.Attachment = &Attachment{
				ExternalReference: extRef,
			}
		}

		ui.AdditionalDocumentReference = append(ui.AdditionalDocumentReference, ref)
	}
}

// AddBinaryAttachment adds an embedded binary attachment to the UBL Invoice.
// This is useful for including documents like PDFs directly within the UBL XML.
// The binary data will be automatically base64-encoded.
func (ui *Invoice) AddBinaryAttachment(attachment BinaryAttachment) {
	ref := Reference{
		ID: IDType{
			Value: attachment.ID,
		},
	}

	if attachment.Description != "" {
		ref.DocumentDescription = attachment.Description
	}

	// Base64-encode the binary data
	encodedData := base64.StdEncoding.EncodeToString(attachment.Data)

	binaryObj := &BinaryObject{
		Value: encodedData,
	}

	if attachment.MimeCode != "" {
		binaryObj.MimeCode = &attachment.MimeCode
	}

	if attachment.Filename != "" {
		binaryObj.Filename = &attachment.Filename
	}

	if attachment.CharacterSetCode != "" {
		binaryObj.CharacterSetCode = &attachment.CharacterSetCode
	}

	if attachment.URI != "" {
		binaryObj.URI = &attachment.URI
	}

	ref.Attachment = &Attachment{
		EmbeddedDocumentBinaryObject: binaryObj,
	}

	ui.AdditionalDocumentReference = append(ui.AdditionalDocumentReference, ref)
}
