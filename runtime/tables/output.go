package tables

import (
	"io"
	"strings"

	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// setPagination implements the Pagination method, which configures the terminal
// dimensions used when rendering paginated output.  Setting both values to zero
// disables pagination (the default for newly created tables).
//
// The parameter order matches the underlying app-cli/tables.SetPagination
// signature: the first argument is height (number of terminal rows) and the
// second is width (number of terminal columns).
func setPagination(s *symbols.SymbolTable, args data.List) (any, error) {
	h, err := data.Int(args.Get(0))
	if err != nil {
		return nil, errors.ErrInvalidInteger.In("SetPagination")
	}

	w, err := data.Int(args.Get(1))
	if err != nil {
		return nil, errors.ErrInvalidInteger.In("SetPagination")
	}

	t, err := getTable(s)
	if err != nil {
		return nil, errors.New(err).In("SetPagination")
	}

	t.SetPagination(h, w)

	return true, err
}

// setFormat implements the Format method, which controls whether the table
// header row and the underline row (===) are included in printed output.
//
// Call patterns:
//   - Format()          → both headings and underlines enabled (defaults)
//   - Format(false)     → both headings and underlines disabled
//   - Format(true, false) → headings shown, underline suppressed
func setFormat(s *symbols.SymbolTable, args data.List) (any, error) {
	t, err := getTable(s)
	if err == nil {
		headings := true
		lines := true

		if args.Len() > 0 {
			headings, err = data.Bool(args.Get(0))
			if err != nil {
				return nil, errors.New(err).In("SetFormat")
			}

			lines = headings
		}

		if args.Len() > 1 {
			lines, err = data.Bool(args.Get(1))
			if err != nil {
				return nil, errors.New(err).In("SetFormat")
			}
		}

		t.ShowHeadings(headings)
		t.ShowUnderlines(lines)
	}

	return err, err
}

// setAlignment implements the Align method, which sets the text alignment for
// a column.  The column may be identified either by name (string) or by its
// zero-based integer index.  The alignment string must be one of "left",
// "right", or "center" (case-insensitive).
func setAlignment(s *symbols.SymbolTable, args data.List) (any, error) {
	t, err := getTable(s)
	if err == nil {
		column := 0

		if columnName, ok := args.Get(0).(string); ok {
			column, ok = t.Column(columnName)
			if !ok {
				err = errors.ErrInvalidColumnName.Context(columnName).In("SetAlignment")

				return err, err
			}
		} else {
			column, err = data.Int(args.Get(0))
			if err != nil {
				return nil, errors.New(err).In("SetAlignment")
			}
		}

		mode := tables.AlignmentLeft

		if modeName, ok := args.Get(1).(string); ok {
			switch strings.ToLower(modeName) {
			case "left":
				mode = tables.AlignmentLeft

			case "right":
				mode = tables.AlignmentRight

			case "center":
				mode = tables.AlignmentCenter

			default:
				err = errors.ErrAlignment.Context(modeName)

				return err, err
			}
		}

		err = t.SetAlignment(column, mode)
	}

	return err, err
}

// printTable implements the Print method, which renders the table and sends it
// to stdout.  The optional format argument overrides the global output format
// (ui.OutputFormat) and accepts "text", "json", or "indented".
//
// If the symbol table contains a defs.StdoutWriterSymbol entry (an io.Writer),
// the formatted text is written there instead of directly to stdout.  The REST
// server uses this mechanism to capture table output into HTTP response bodies.
func printTable(s *symbols.SymbolTable, args data.List) (any, error) {
	fmt := ui.OutputFormat

	if args.Len() > 0 {
		fmt = data.String(args.Get(0))
	}

	t, err := getTable(s)
	if err != nil {
		return err, err
	}

	// Do we have a writer we should be using other than stdout? If so,
	// it's been stashed in the symbol table. If found, format the table
	// as text and then send the text to the writer.
	if writer, found := s.Get(defs.StdoutWriterSymbol); found {
		if writer, ok := writer.(io.Writer); ok {
			text, err := t.String(fmt)
			if err != nil {
				return err, err
			}

			_, err = writer.Write([]byte(data.String(text)))

			return err, err
		}
	}

	// No back-channel writer found, so just ask the table to print
	// itself to the default stdout.
	err = t.Print(fmt)

	return err, err
}

// toString implements the String method, which formats the table and returns
// the result as a Go string rather than writing it to any output stream.  The
// optional format argument works the same as for printTable.
func toString(s *symbols.SymbolTable, args data.List) (any, error) {
	fmt := ui.OutputFormat

	if args.Len() > 0 {
		fmt = data.String(args.Get(0))
	}

	t, err := getTable(s)
	if err == nil {
		return t.String(fmt)
	}

	return nil, err
}
