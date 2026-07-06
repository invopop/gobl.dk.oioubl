package dkoioubl

import (
	"encoding/base64"
	"regexp"

	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/dsig"
	"github.com/invopop/gobl/org"
)

// goblAddAttachments processes all attachments from the UBL Invoice and returns
// external reference attachments only.
// Binary attachments are skipped - use ExtractBinaryAttachments to retrieve them.
func (ui *Invoice) goblAddAttachments() []*org.Attachment {
	var attachments []*org.Attachment

	for _, ref := range ui.AdditionalDocumentReference {
		if ref.Attachment == nil {
			continue
		}

		// Only process external reference attachments
		if ref.Attachment.ExternalReference != nil {
			att := ui.processExternalAttachment(&ref)
			if att != nil {
				attachments = append(attachments, att)
			}
		}
		// Binary attachments are skipped - handled by ExtractBinaryAttachments
	}

	return attachments
}

// processExternalAttachment converts a UBL external reference attachment to GOBL format.
func (ui *Invoice) processExternalAttachment(ref *Reference) *org.Attachment {
	extRef := ref.Attachment.ExternalReference
	if extRef == nil {
		return nil
	}

	att := &org.Attachment{
		URL:  extRef.URI,
		MIME: extRef.MimeCode,
	}

	if extRef.DocumentHash != "" && extRef.HashAlgorithmMethod != "" {
		att.Digest = &dsig.Digest{
			Value:     extRef.DocumentHash,
			Algorithm: dsig.DigestAlgorithm(extRef.HashAlgorithmMethod),
		}
	}

	att.Code = cbc.Code(cleanString(ref.ID.Value))
	att.Description = cleanString(ref.DocumentDescription)

	return att
}

// ExtractBinaryAttachments extracts all binary attachments from the UBL Invoice.
// It returns a slice of BinaryAttachment containing the ID, description, and decoded binary data.
// External reference attachments are not included in the result.
func (ui *Invoice) ExtractBinaryAttachments() []BinaryAttachment {
	var result []BinaryAttachment

	for _, ref := range ui.AdditionalDocumentReference {
		if ref.Attachment != nil && ref.Attachment.EmbeddedDocumentBinaryObject != nil {
			binObj := ref.Attachment.EmbeddedDocumentBinaryObject

			// Decode the base64 data
			var data []byte
			if binObj.Value != "" {
				// Remove whitespace that may have been added by XML formatting
				whitespaceRegex := regexp.MustCompile(`\s+`)
				v := whitespaceRegex.ReplaceAllString(binObj.Value, "")

				decoded, err := base64.StdEncoding.DecodeString(v)
				if err != nil {
					continue
				}

				data = decoded

			}

			attachment := BinaryAttachment{
				ID:          ref.ID.Value,
				Description: ref.DocumentDescription,
				Data:        data,
			}

			if binObj.MimeCode != nil {
				attachment.MimeCode = *binObj.MimeCode
			}
			if binObj.Filename != nil {
				attachment.Filename = *binObj.Filename
			}
			if binObj.CharacterSetCode != nil {
				attachment.CharacterSetCode = *binObj.CharacterSetCode
			}
			if binObj.URI != nil {
				attachment.URI = *binObj.URI
			}

			result = append(result, attachment)
		}
	}

	return result
}
