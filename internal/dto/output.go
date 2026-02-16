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

func ParseOutputFormat(raw string) OutputFormat {
	if strings.EqualFold(raw, string(JSON)) {
		return JSON
	}

	return Natural
}
