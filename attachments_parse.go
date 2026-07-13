package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
)

// ExtractBinaryAttachments returns the invoice's decoded embedded binary attachments.
func (ui *Invoice) ExtractBinaryAttachments() []BinaryAttachment {
	return (*ubl.Invoice)(ui).ExtractBinaryAttachments()
}
