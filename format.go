package oioubl

import (
	"strings"
	"time"

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

// normalizeNumericString preps a wire amount for num parsing: trims space, zero-pads a leading decimal point.
func normalizeNumericString(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, ".") {
		s = "0" + s
	}
	return s
}

// parseWireDate reads a UBL YYYY-MM-DD date.
func parseWireDate(s string) (cal.Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return cal.Date{}, err
	}
	return cal.MakeDate(t.Year(), t.Month(), t.Day()), nil
}
