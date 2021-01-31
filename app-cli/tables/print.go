package tables

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/expressions"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Print will output a table using current rows and format specifications.
func (t *Table) Print(format string) error {

	// If there is an orderBy set for the table, do the sort now
	if t.orderBy >= 0 {
		_ = t.SortRows(t.orderBy, t.ascending)
	}

	if format == "" {
		format = ui.TextFormat
	}

	// Based on the selected format, generate the output
	switch format {
	case ui.TextFormat:
		s := t.FormatText()
		for _, line := range s {
			fmt.Printf("%s\n", line)
		}

	case ui.JSONFormat:
		fmt.Printf("%s\n", t.FormatJSON())

	case ui.JSONIndentedFormat:
		text := t.FormatJSON()
		var i interface{}
		_ = json.Unmarshal([]byte(text), &i)
		b, _ := json.MarshalIndent(i, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
		fmt.Printf("%s\n", string(b))

	default:
		return errors.New("Invalid table format value")
	}
	return nil
}

// FormatJSON will produce the text of the table as JSON
func (t *Table) FormatJSON() string {

	var buffer strings.Builder
	var e *expressions.Expression
	var firstRow = true

	if t.where != "" {
		e = expressions.New().WithText(t.where)
	}

	buffer.WriteRune('[')
	for n, row := range t.rows {
		if n < t.startingRow {
			continue
		}
		if t.rowLimit > 0 && n >= t.startingRow+t.rowLimit {
			break
		}

		if e != nil {
			// Load up the symbol tables with column values and the row number
			symbols := symbols.NewSymbolTable("rowset")
			_ = symbols.SetAlways("_row_", n+1)
			for i, n := range t.columns {
				_ = symbols.SetAlways(strings.ToLower(n), row[i])
			}
			v, err := e.Eval(symbols)
			if err != nil {
				buffer.WriteString(fmt.Sprintf("*** where clause error: %s", err.Error()))
				break
			}
			if !util.GetBool(v) {
				continue
			}
		}

		if !firstRow {
			buffer.WriteRune(',')
		}
		firstRow = false
		buffer.WriteRune('{')
		for ith, i := range t.columnOrder {
			header := t.columns[i]
			if ith > 0 {
				buffer.WriteRune(',')
			}
			buffer.WriteRune('"')
			buffer.WriteString(header)
			buffer.WriteString("\":")

			if _, valid := strconv.Atoi(row[i]); valid == nil {
				buffer.WriteString(row[i])
			} else if row[i] == "true" || row[i] == "false" {
				buffer.WriteString(row[i])
			} else {
				buffer.WriteString("\"" + row[i] + "\"")
			}
		}
		buffer.WriteRune('}')

	}
	buffer.WriteRune(']')
	return buffer.String()
}

// FormatText will output a table using current rows and format specifications.
func (t *Table) FormatText() []string {

	ui.Debug(ui.AppLogger, "Print column order: %v", t.columnOrder)
	output := make([]string, 0)

	var e *expressions.Expression
	if t.where != "" {
		e = expressions.New().WithText(t.where)
		if ui.DebugMode {
			e.Disasm()
		}
	}

	var buffer strings.Builder
	var rowLimit = t.rowLimit
	if rowLimit < 0 {
		rowLimit = len(t.rows)
	}

	if t.showHeadings {
		buffer.WriteString(t.indent)
		if t.showRowNumbers {
			buffer.WriteString("Row")
			buffer.WriteString(t.spacing)
		}
		for _, n := range t.columnOrder {
			buffer.WriteString(AlignText(t.columns[n], t.maxWidth[n], t.alignment[n]))
			buffer.WriteString(t.spacing)
		}
		output = append(output, buffer.String())

		if t.showUnderlines {
			buffer.Reset()
			buffer.WriteString(t.indent)
			if t.showRowNumbers {
				buffer.WriteString("===")
				buffer.WriteString(t.spacing)
			}
			for _, n := range t.columnOrder {
				for pad := 0; pad < t.maxWidth[n]; pad++ {
					buffer.WriteRune('=')
				}
				buffer.WriteString(t.spacing)
			}
			output = append(output, buffer.String())
		}
	}

	for i, r := range t.rows {

		if i < t.startingRow {
			continue
		}
		if i >= t.startingRow+rowLimit {
			break
		}

		buffer.Reset()
		buffer.WriteString(t.indent)

		if e != nil {
			// Load up the symbol tables with column values and the row number
			symbols := symbols.NewSymbolTable("rowset")
			_ = symbols.SetAlways("_row_", i+1)
			for i, n := range t.columns {
				_ = symbols.SetAlways(strings.ToLower(n), r[i])
			}
			v, err := e.Eval(symbols)
			if err != nil {
				output = append(output, fmt.Sprintf("*** where clause error: %s", err.Error()))
				break
			}
			if !util.GetBool(v) {
				continue
			}
		}
		if t.showRowNumbers {
			buffer.WriteString(fmt.Sprintf("%3d", i+1))
			buffer.WriteString(t.spacing)
		}

		// Loop over the elements of the row. Generate pre- or post-spacing as
		// appropriate for the requested alignment, and any intra-column spacing.
		for _, n := range t.columnOrder {
			buffer.WriteString(AlignText(r[n], t.maxWidth[n], t.alignment[n]))
			buffer.WriteString(t.spacing)
		}
		output = append(output, buffer.String())
	}

	return output
}

func AlignText(text string, width int, alignment int) string {

	if len(text) >= width {
		switch alignment {
		case AlignmentLeft:
			return text[:width]

		case AlignmentRight:
			return text[len(text)-width:]
		case AlignmentCenter:
			pos := len(text)/2 - (width / 2)
			return text[pos : pos+width]

		default:
			return text[:width]
		}
	}
	pad := strings.Repeat(" ", width)
	switch alignment {
	case AlignmentRight:
		r := pad + text
		return r[len(r)-width:]

	case AlignmentLeft:
		r := text + pad
		return r[:width]

	case AlignmentCenter:
		r := text
		left := true
		for len(r) < width {
			if left {
				r = " " + r
			} else {
				r = r + " "
			}
			left = !left
		}
		return r

	default: // same as AlignmentLeft
		r := text + pad
		return r[:width]
	}
}
