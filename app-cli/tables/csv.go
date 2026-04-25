package tables

import (
	"strings"
)

// NewCSV creates a new table by parsing a comma-separated list of column
// headings from the single string h. It is equivalent to calling
// New(CsvSplit(h)) and is convenient when the headings come from the
// first line of a CSV file.
func NewCSV(h string) (*Table, error) {
	return New(CsvSplit(h))
}

// AddCSVRow appends a data row to the table by parsing a comma-separated
// string. The number of fields produced by CsvSplit must equal the table's
// column count; otherwise AddRow returns ErrColumnCount.
func (t *Table) AddCSVRow(items string) error {
	return t.AddRow(CsvSplit(items))
}

// CsvSplit splits a comma-separated string into a slice of trimmed field
// strings. Double-quote characters toggle "in-quote" mode: commas inside
// quotes are treated as literal characters, not separators. The quote
// characters themselves are not included in the output.
//
// Edge cases:
//   - An empty input string returns nil (not an empty slice).
//   - A trailing comma without a following value does not produce an extra
//     empty element (only non-empty accumulated text is appended).
//   - Spaces around values are trimmed with strings.TrimSpace.
//
// Example:
//
//	CsvSplit(`"Name,Age", City`) → []string{"Name,Age", "City"}
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
