package oioubl

import (
	"github.com/invopop/gobl/cal"
)

// ptr addresses a string, which most optional UBL fields need.
func ptr(s string) *string {
	return &s
}

// formatDate renders a GOBL date in UBL's YYYY-MM-DD form.
func formatDate(d cal.Date) string {
	if d.IsZero() {
		return ""
	}
	return d.Time().Format("2006-01-02")
}
