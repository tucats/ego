package tables

import (
	"strings"
	"testing"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
)

func TestTable_FormatJSON(t *testing.T) {
	type fields struct {
		rows           [][]string
		columns        []string
		alignment      []int
		maxWidth       []int
		columnOrder    []int
		spacing        string
		indent         string
		rowLimit       int
		startingRow    int
		columnCount    int
		rowCount       int
		orderBy        int
		ascending      bool
		showUnderlines bool
		showHeadings   bool
		showRowNumbers bool
	}

	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "Single simple column",
			fields: fields{
				columnCount: 1,
				columns:     []string{"one"},
				rows:        [][]string{{"1"}},
				columnOrder: []int{0},
			},
			want: "[{\"one\":1}]",
		},
		{
			name: "Three columns of int, bool, string types",
			fields: fields{
				columnCount: 3,
				columns:     []string{"one", "two", "three"},
				rows:        [][]string{{"1", defs.True, "Tom"}},
				columnOrder: []int{0, 1, 2},
			},
			want: "[{\"one\":1,\"two\":true,\"three\":\"Tom\"}]",
		},
		{
			name: "Two rows of two columns",
			fields: fields{
				columnCount: 3,
				columns:     []string{"one", "two"},
				columnOrder: []int{0, 1},
				rows: [][]string{
					{"60", "Tom"},
					{"59", "Mary"},
				},
			},
			want: "[{\"one\":60,\"two\":\"Tom\"},{\"one\":59,\"two\":\"Mary\"}]",
		},
		{
			// Regression test: a column header containing a double-quote
			// character (e.g. a crafted SQL column alias such as
			// `SELECT 1 AS "a""b"`) must be escaped exactly like a value is,
			// not written into the output raw. Before this fix, only values
			// were escaped, so a header like this would break the JSON
			// structure instead of producing a single well-formed key.
			name: "column header containing a double quote is escaped",
			fields: fields{
				columnCount: 1,
				columns:     []string{`a"b`},
				rows:        [][]string{{"Tom"}},
				columnOrder: []int{0},
			},
			want: `[{"a\"b":"Tom"}]`,
		},
		{
			// Regression test: a value containing a double-quote must be
			// escaped exactly once. Before this fix, FormatJSON ran the
			// value through a redundant escape() helper before strconv.Quote,
			// which double-escaped the quote (producing \\\" instead of \")
			// and corrupted the value on the round trip through a JSON parser.
			name: "value containing a double quote is escaped exactly once",
			fields: fields{
				columnCount: 1,
				columns:     []string{"quip"},
				rows:        [][]string{{`he said "hi"`}},
				columnOrder: []int{0},
			},
			want: `[{"quip":"he said \"hi\""}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &Table{
				showUnderlines: tt.fields.showUnderlines,
				showHeadings:   tt.fields.showHeadings,
				showRowNumbers: tt.fields.showRowNumbers,
				rowLimit:       tt.fields.rowLimit,
				startingRow:    tt.fields.startingRow,
				columnCount:    tt.fields.columnCount,
				orderBy:        tt.fields.orderBy,
				ascending:      tt.fields.ascending,
				rows:           tt.fields.rows,
				names:          tt.fields.columns,
				columnOrder:    tt.fields.columnOrder,
				alignment:      tt.fields.alignment,
				maxWidth:       tt.fields.maxWidth,
				spacing:        tt.fields.spacing,
				indent:         tt.fields.indent,
			}
			if got := tx.FormatJSON(); got != tt.want {
				t.Errorf("Table.FormatJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAlignText(t *testing.T) {
	type args struct {
		text      string
		width     int
		alignment int
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "simple left alignment with pad",
			args: args{text: "hello", width: 10, alignment: AlignmentLeft},
			want: "hello     ",
		},
		{
			name: "simple right alignment with pad",
			args: args{text: "hello", width: 10, alignment: AlignmentRight},
			want: "     hello",
		},
		{
			name: "simple center alignment with pad",
			args: args{text: "hello", width: 9, alignment: AlignmentCenter},
			want: "  hello  ",
		},
		{
			name: "truncated left alignment",
			args: args{text: "hello", width: 3, alignment: AlignmentLeft},
			want: "hel",
		},
		{
			name: "truncated right alignment",
			args: args{text: "hello", width: 3, alignment: AlignmentRight},
			want: "llo",
		},
		{
			name: "truncated left alignment",
			args: args{text: "hello", width: 3, alignment: AlignmentCenter},
			want: "ell",
		},
		{
			// Note the text has a multi-byte first character.
			name: "multi-byte unicode left alignment",
			args: args{text: "Ąnswer", width: 10, alignment: AlignmentLeft},
			want: "Ąnswer    ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AlignText(tt.args.text, tt.args.width, tt.args.alignment); got != tt.want {
				t.Errorf("AlignText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTable_paginateText(t *testing.T) {
	t.Run("header test", func(t *testing.T) {
		// The expected values below were written for the old paginator.
		// They need to be updated to match the new paginator (RenderPagelets)
		// before this sub-test can be re-enabled.
		t.Skip("expected values not yet updated for new paginator; needs rework")

		tb, _ := New([]string{"A", "B", "C"})
		tb.SetPagination(3, 80)
		tb.ShowRowNumbers(true)

		_ = tb.AddRowItems("Axel", "Balls", "Chuckle")
		_ = tb.AddRowItems("Apes", "Bugs", "Cows")
		_ = tb.AddRowItems("Apple", "Berry", "Cuke")
		_ = tb.AddRowItems("Able", "Baker", "Charlie")
		_ = tb.AddRowItems("Ant", "Beetle", "Cockroach")

		text, err := tb.String(ui.TextFormat)
		if err != nil {
			t.Error("Error formatting paginated rows,", err.Error())
		}

		expected := []string{
			`Row  A      B       C`,
			`===  =====  ======  =========`,
			`  1  Axel   Balls   Chuckle`,
			`  2  Apes   Bugs    Cows`,
			``,
			`Row  A      B       C`,
			`===  =====  ======  =========`,
			`  3  Apple  Berry   Cuke`,
			`  4  Able   Baker   Charlie`,
			``,
			`Row  A      B       C`,
			`===  =====  ======  =========`,
			`  5  Ant    Beetle  Cockroach`,
			``,
			``,
		}

		lines := strings.Split(text, "\n")

		for i, line := range lines {
			got := line
			for strings.HasSuffix(got, " ") {
				got = strings.TrimSuffix(got, " ")
			}

			if i >= len(expected) {
				t.Errorf("Result has more lines than expected, got line %d\n%s", i, got)
			} else if expected[i] != got {
				t.Errorf(" line %d\n%s", i, got)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// FormatJSON — empty table and boolean values
// ---------------------------------------------------------------------------

func TestTable_FormatJSON_EmptyTable(t *testing.T) {
	tb, _ := New([]string{"a", "b"})
	tb.SetPagination(0, 0)

	got := tb.FormatJSON()
	if got != "[]" {
		t.Errorf("FormatJSON on empty table = %q, want %q", got, "[]")
	}
}

func TestTable_FormatJSON_BoolValues(t *testing.T) {
	tb, _ := New([]string{"flag"})
	tb.SetPagination(0, 0)
	_ = tb.AddRow([]string{defs.True})
	_ = tb.AddRow([]string{defs.False})

	got := tb.FormatJSON()
	if !strings.Contains(got, ":true") {
		t.Errorf("FormatJSON: expected bare true in JSON, got %q", got)
	}

	if !strings.Contains(got, ":false") {
		t.Errorf("FormatJSON: expected bare false in JSON, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// FormatIndented — header and value escaping
// ---------------------------------------------------------------------------

// TestTable_FormatIndented_EscapesHeaderAndValue is the FormatIndented
// counterpart of the two new FormatJSON cases above: FormatIndented shares
// the same header/value-writing logic, so it needs the same regression
// coverage for (a) a header containing a double quote, and (b) a value
// containing a double quote being escaped exactly once (not left raw, and
// not double-escaped).
func TestTable_FormatIndented_EscapesHeaderAndValue(t *testing.T) {
	tb, err := New([]string{`a"b`})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tb.SetPagination(0, 0)

	if err := tb.AddRow([]string{`he said "hi"`}); err != nil {
		t.Fatalf("AddRow() error = %v", err)
	}

	got := tb.FormatIndented()

	if !strings.Contains(got, `"a\"b":`) {
		t.Errorf("FormatIndented() = %q, want header escaped as \"a\\\"b\":", got)
	}

	if !strings.Contains(got, `"he said \"hi\""`) {
		t.Errorf("FormatIndented() = %q, want value escaped exactly once as \"he said \\\"hi\\\"\"", got)
	}

	// A double-escaped value (the pre-fix bug) would contain a literal
	// backslash-backslash sequence; make sure that never appears.
	if strings.Contains(got, `\\\"`) {
		t.Errorf("FormatIndented() = %q, value appears to be double-escaped", got)
	}
}

func TestTable_FormatIndented_EmptyTable(t *testing.T) {
	tb, _ := New([]string{"x"})
	tb.SetPagination(0, 0)

	got := tb.FormatIndented()
	// An empty table should produce the array brackets with no elements.
	if !strings.HasPrefix(got, "[\n") || !strings.Contains(got, "]\n") {
		t.Errorf("FormatIndented empty table = %q, want [\\n...\\n]\\n", got)
	}

	if strings.Contains(got, "{") {
		t.Errorf("FormatIndented empty table should contain no objects, got: %q", got)
	}
}

// ---------------------------------------------------------------------------
// FormatText — empty table (headings only) and indent effect
// ---------------------------------------------------------------------------

func TestTable_FormatText_EmptyTable(t *testing.T) {
	tb, _ := New([]string{"col"})
	tb.SetPagination(0, 0)

	lines := tb.FormatText()
	// Headings and underline are shown by default even with no data rows.
	if len(lines) < 2 {
		t.Fatalf("FormatText on empty table: expected at least 2 lines (heading + underline), got %d", len(lines))
	}

	if !strings.Contains(lines[0], "col") {
		t.Errorf("FormatText empty table: heading line missing 'col': %q", lines[0])
	}
}

func TestTable_FormatText_Indent(t *testing.T) {
	tb, _ := New([]string{"v"})
	tb.SetPagination(0, 0)
	_ = tb.AddRow([]string{"data"})
	_ = tb.SetIndent(4)

	lines := tb.FormatText()
	for _, l := range lines {
		if l != "" && !strings.HasPrefix(l, "    ") {
			t.Errorf("SetIndent(4): line %q does not start with 4 spaces", l)
		}
	}
}

// ---------------------------------------------------------------------------
// String — JSONIndented format
// ---------------------------------------------------------------------------

func TestTable_String_JSONIndented(t *testing.T) {
	tb, _ := New([]string{"name", "score"})
	tb.SetPagination(0, 0)
	_ = tb.AddRow([]string{"Alice", "42"})

	got, err := tb.String(ui.JSONIndentedFormat)
	if err != nil {
		t.Fatalf("String(JSONIndentedFormat) error: %v", err)
	}

	if !strings.Contains(got, "\"name\"") {
		t.Errorf("String(JSONIndented) missing key 'name': %q", got)
	}

	if !strings.Contains(got, "\"Alice\"") {
		t.Errorf("String(JSONIndented) missing value 'Alice': %q", got)
	}

	// Numeric value should be emitted without quotes.
	if !strings.Contains(got, "42") {
		t.Errorf("String(JSONIndented) missing numeric value 42: %q", got)
	}

	// Should be formatted with newlines.
	if !strings.Contains(got, "\n") {
		t.Errorf("String(JSONIndented) should contain newlines: %q", got)
	}
}

// ---------------------------------------------------------------------------
// AlignText — additional edge cases
// ---------------------------------------------------------------------------

func TestAlignText_ZeroWidth(t *testing.T) {
	// A width of 0 truncates all text to the first 0 runes — empty string.
	got := AlignText("hello", 0, AlignmentLeft)
	if got != "" {
		t.Errorf("AlignText with width=0: got %q, want \"\"", got)
	}
}

func TestAlignText_ExactWidth(t *testing.T) {
	// When text length equals width, no padding or truncation should occur.
	got := AlignText("hello", 5, AlignmentLeft)
	if got != "hello" {
		t.Errorf("AlignText exact width left: got %q, want %q", got, "hello")
	}

	got = AlignText("hello", 5, AlignmentRight)
	if got != "hello" {
		t.Errorf("AlignText exact width right: got %q, want %q", got, "hello")
	}
}

// Previously, AddColumn always initialized maxWidth to 0 instead of the
// heading's rune length, causing the heading to be truncated in FormatText
// when all row values were shorter than the heading. AddColumn now counts
// the heading's runes and stores that as the initial maxWidth.
func TestTable_AddColumn_HeadingNotTruncated(t *testing.T) {
	tb, _ := New([]string{"a"})
	_ = tb.AddColumn("VeryLong")
	_ = tb.AddRow([]string{"v1", "v2"}) // "v2" is 2 runes — shorter than "VeryLong" (8)

	tb.SetPagination(0, 0)

	lines := tb.FormatText()
	if len(lines) == 0 {
		t.Fatal("FormatText returned no lines")
	}

	if !strings.Contains(lines[0], "VeryLong") {
		t.Errorf("AddColumn heading truncated: heading line = %q, want it to contain \"VeryLong\"", lines[0])
	}
}
