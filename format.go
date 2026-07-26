package dkoioubl

import (
	"strings"

	"github.com/invopop/gobl/cal"
)

// formatDate renders a GOBL date in UBL's YYYY-MM-DD form.
func formatDate(d cal.Date) string {
	if d.IsZero() {
		return ""
	}
	return d.Time().Format("2006-01-02")
}

// normalizeNumericString preps a wire amount for num parsing: trims space, zero-pads a leading decimal point.
func normalizeNumericString(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, ".") {
		s = "0" + s
	}
	return s
}
