package tables

import (
	"strings"
)

// NewCSV creates a new table using a single string with comma-separated
// heading names. These typically correspond to the first row in a CSV
// data file.
func NewCSV(h string) (*Table, error) {
	return New(CsvSplit(h))
}

// AddCSVRow adds a row to an existing table, where the row is expressed
// as a string with comma-separated values.
func (t *Table) AddCSVRow(items string) error {
	return t.AddRow(CsvSplit(items))
}

// CsvSplit takes a line that is comma-separated and splits it into
// an array of strings. Quoted commas are ignored as separators. The
// values are trimmed of extra spaces.
func CsvSplit(data string) []string {
	var (
		headings       []string
		inQuote        = false
		currentHeading strings.Builder
	)

	for _, c := range data {
		if c == '"' {
			inQuote = !inQuote

			continue
		}

		if !inQuote && c == ',' {
			headings = append(headings, strings.TrimSpace(currentHeading.String()))
			currentHeading.Reset()

			continue
		}

		currentHeading.WriteRune(c)
	}

	if currentHeading.Len() > 0 {
		headings = append(headings, strings.TrimSpace(currentHeading.String()))
	}

	return headings
}
