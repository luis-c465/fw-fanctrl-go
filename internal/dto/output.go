package dto

import "strings"

type OutputFormat string

const (
	Natural OutputFormat = "NATURAL"
	JSON    OutputFormat = "JSON"
)

type Formattable interface {
	ToOutputFormat(format OutputFormat) string
	ToJSON() string
}

// Printable mirrors the legacy Python naming and remains available for
// compatibility with packages/tests that still refer to the old type name.
type Printable = Formattable

func ParseOutputFormat(raw string) OutputFormat {
	if strings.EqualFold(raw, string(JSON)) {
		return JSON
	}

	return Natural
}
