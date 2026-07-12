package dkoioubl

import (
	ubl "github.com/invopop/gobl.ubl"
)

// ExtractBinaryAttachments returns the invoice's decoded embedded binary
// attachments (external document references excluded); gov-dk uses it on parse.
func (ui *Invoice) ExtractBinaryAttachments() []BinaryAttachment {
	return (*ubl.Invoice)(ui).ExtractBinaryAttachments()
}
