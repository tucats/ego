package tables

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

func TestTable_SortRows(t *testing.T) {
	tests := []struct {
		name        string
		headers     []string
		rows        [][]string
		sortColumn  string
		result      []string
		startingRow int
		width       int
		wantErr     bool
		hideLines   bool
		hideHeaders bool
	}{
		{
			name:       "Simple table with one column, two rows",
			headers:    []string{"first"},
			rows:       [][]string{{"v2"}, {"v1"}},
			sortColumn: "first",
			result:     []string{"first    ", "=====    ", "v1       ", "v2       "},
		},
		{
			name:       "Simple table with two columns, two rows",
			headers:    []string{"first", "second"},
			rows:       [][]string{{"v2", "d1"}, {"v1", "d2"}},
			sortColumn: "first",
			result:     []string{"first    second    ", "=====    ======    ", "v1       d2        ", "v2       d1        "},
			wantErr:    false,
		},
		{
			name:       "Simple table with two columns, two rows, alternate sort",
			headers:    []string{"first", "second"},
			rows:       [][]string{{"v2", "d1"}, {"v1", "d2"}},
			sortColumn: "second",
			result:     []string{"first    second    ", "=====    ======    ", "v2       d1        ", "v1       d2        "},
			wantErr:    false,
		},
		{
			name:       "Simple table with two columns, two rows, alternate descending sort",
			headers:    []string{"first", "second"},
			rows:       [][]string{{"v2", "d1"}, {"v1", "d2"}},
			sortColumn: "~second",
			result:     []string{"first    second    ", "=====    ======    ", "v1       d2        ", "v2       d1        "},
			wantErr:    false,
		},
		{
			name:       "Simple table with two columns, two rows, invalid sort",
			headers:    []string{"first", "second"},
			rows:       [][]string{{"v2", "d1"}, {"v1", "d2"}},
			sortColumn: "foobar",
			result:     []string{"first    second    ", "=====    ======    ", "v2       d1        ", "v1       d2        "},
			wantErr:    true,
		},
		{
			name:        "Starting row",
			headers:     []string{"first", "second"},
			rows:        [][]string{{"v2", "d1"}, {"v1", "d2"}},
			sortColumn:  "second",
			result:      []string{"first    second    ", "=====    ======    ", "v1       d2        "},
			startingRow: 2,
		},
		{
			name:        "Invalid starting row",
			headers:     []string{"first", "second"},
			rows:        [][]string{{"v2", "d1"}, {"v1", "d2"}},
			sortColumn:  "second",
			result:      []string{"first    second    ", "=====    ======    ", "v2       d1        ", "v1       d2        "},
			startingRow: -5,
			wantErr:     true,
		},
		{
			name:       "Format table without underlines",
			headers:    []string{"first", "second"},
			rows:       [][]string{{"v2", "d1"}, {"v1", "d2"}},
			sortColumn: "second",
			result:     []string{"first    second    ", "v2       d1        ", "v1       d2        "},
			hideLines:  true,
		},
		{
			name:        "Format table without headings",
			headers:     []string{"first", "second"},
			rows:        [][]string{{"v2", "d1"}, {"v1", "d2"}},
			sortColumn:  "second",
			result:      []string{"v2       d1        ", "v1       d2        "},
			hideHeaders: true,
		},
		{
			name:        "Valid minimum width",
			headers:     []string{"first", "second"},
			rows:        [][]string{{"v2", "d1"}, {"v1", "d2"}},
			sortColumn:  "second",
			result:      []string{"v2       d1            ", "v1       d2            "},
			hideHeaders: true,
			width:       10,
		},
		{
			name:        "Valid but ineffective width",
			headers:     []string{"first", "second"},
			rows:        [][]string{{"v2", "d1"}, {"v1", "d2"}},
			sortColumn:  "second",
			result:      []string{"v2       d1        ", "v1       d2        "},
			width:       2,
			hideHeaders: true,
		},
		{
			name:        "Invalid minimum width",
			headers:     []string{"first", "second"},
			rows:        [][]string{{"v2", "d1"}, {"v1", "d2"}},
			sortColumn:  "second",
			result:      []string{"v2       d1        ", "v1       d2        "},
			hideHeaders: true,
			wantErr:     true,
			width:       -3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table, _ := New(tt.headers)
			for _, r := range tt.rows {
				_ = table.AddRow(r)
			}

			sortTable(tt, table, t)

			if tt.hideLines {
				table.ShowUnderlines(false)
			}

			if tt.hideHeaders {
				table.ShowHeadings(false)
			}

			if tt.startingRow != 0 {
				err := table.SetStartingRow(tt.startingRow)
				if (err != nil) && !tt.wantErr {
					t.Errorf("Unexpected SetStartingRow error result: %v", err)
				}
			}

			if tt.width != 0 {
				err := table.SetMinimumWidth(1, tt.width)
				if (err != nil) && !tt.wantErr {
					t.Errorf("Unexpected SetMinimumWidth error result: %v", err)
				}
			}

			x := table.FormatText()

			if !reflect.DeepEqual(x, tt.result) {
				t.Errorf("Sorted row results wrong. Got %v want %v", x, tt.result)
			}
		})
	}
}

func sortTable(tt struct{name string; headers []string; rows [][]string; sortColumn string; result []string; startingRow int; width int; wantErr bool; hideLines bool; hideHeaders bool}, table *Table, t *testing.T) {
	if tt.sortColumn != "" {
		err := table.SetOrderBy(tt.sortColumn)
		if (err != nil) && !tt.wantErr {
			t.Errorf("Unexpected SetOrderBy error result: %v", err)
		}

		err = table.SortRows(table.orderBy, table.ascending)
		if (err != nil) && !tt.wantErr {
			t.Errorf("Unexpected SortRows error result: %v", err)
		}
	}
}

func TestTable_AddRow(t *testing.T) {
	type args struct {
		row []string
	}

	tests := []struct {
		name    string
		table   Table
		args    args
		want    Table
		wantErr bool
	}{
		{
			name: "Add one row to a single column",
			table: Table{
				rowLimit:       -1,
				columnCount:    1,
				names:          []string{"simple"},
				maxWidth:       []int{6},
				alignment:      []int{AlignmentLeft},
				spacing:        "    ",
				indent:         "",
				rows:           make([][]string, 0),
				orderBy:        -1,
				ascending:      true,
				showUnderlines: true,
				showHeadings:   true,
			},
			args: args{
				row: []string{"first"},
			},
			want: Table{
				rowLimit:       -1,
				columnCount:    1,
				names:          []string{"simple"},
				maxWidth:       []int{6},
				alignment:      []int{AlignmentLeft},
				spacing:        "    ",
				indent:         "",
				rows:           [][]string{{"first"}},
				orderBy:        -1,
				ascending:      true,
				showUnderlines: true,
				showHeadings:   true,
			},
			wantErr: false,
		},
		{
			name: "Add one row to a three-column table",
			table: Table{
				rowLimit:       -1,
				columnCount:    3,
				names:          []string{"simple", "test", "table"},
				maxWidth:       []int{6, 4, 5},
				alignment:      []int{AlignmentLeft, AlignmentLeft, AlignmentLeft},
				spacing:        "    ",
				indent:         "",
				rows:           make([][]string, 0),
				orderBy:        -1,
				ascending:      true,
				showUnderlines: true,
				showHeadings:   true,
			},
			args: args{
				row: []string{"first", "second", "third"},
			},
			want: Table{
				rowLimit:       -1,
				columnCount:    3,
				names:          []string{"simple", "test", "table"},
				maxWidth:       []int{6, 6, 5},
				alignment:      []int{AlignmentLeft, AlignmentLeft, AlignmentLeft},
				spacing:        "    ",
				indent:         "",
				rows:           [][]string{{"first", "second", "third"}},
				orderBy:        -1,
				ascending:      true,
				showUnderlines: true,
				showHeadings:   true,
			},
			wantErr: false,
		},
		{
			name: "Add two-column row to three-column table",
			table: Table{
				rowLimit:       -1,
				columnCount:    3,
				names:          []string{"simple", "test", "table"},
				maxWidth:       []int{6, 4, 5},
				alignment:      []int{AlignmentLeft, AlignmentLeft, AlignmentLeft},
				spacing:        "    ",
				indent:         "",
				rows:           make([][]string, 0),
				orderBy:        -1,
				ascending:      true,
				showUnderlines: true,
				showHeadings:   true,
			},
			args: args{
				row: []string{"first", "second"},
			},
			want: Table{
				rowLimit:       -1,
				columnCount:    3,
				names:          []string{"simple", "test", "table"},
				maxWidth:       []int{6, 4, 5},
				alignment:      []int{AlignmentLeft, AlignmentLeft, AlignmentLeft},
				spacing:        "    ",
				indent:         "",
				rows:           make([][]string, 0),
				orderBy:        -1,
				ascending:      true,
				showUnderlines: true,
				showHeadings:   true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTable := &tt.table

			err := testTable.AddRow(tt.args.row)
			if (err != nil) != tt.wantErr {
				t.Errorf("Table.AddRow() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(*testTable, tt.want) {
				t.Errorf("Table.AddRow() got %v, want %v", testTable, tt.want)
			}
		})
	}
}

func TestTable_AddRowItems(t *testing.T) {
	t.Run("Add row items", func(t *testing.T) {
		tb, _ := New([]string{"Age", "Name", "Active", "Ratio"})
		_ = tb.AddRowItems(60, "Tom", true, 28.5)
		_ = tb.AddRowItems(59, "Mary", true, 23.5)
		_ = tb.AddRowItems(62, "Tony", false, 35.9)
		_ = tb.SortRows(3, false)

		expected := [][]string{
			{"62", "Tony", defs.False, "35.9"},
			{"60", "Tom", defs.True, "28.5"},
			{"59", "Mary", defs.True, "23.5"},
		}
		if !reflect.DeepEqual(tb.rows, expected) {
			t.Errorf("Table.AddRowItems() got %v, want %v", tb.rows, expected)
		}
	})
}

// ---------------------------------------------------------------------------
// AddRowItems — error path
// ---------------------------------------------------------------------------

func TestTable_AddRowItems_WrongCount(t *testing.T) {
	tb, _ := New([]string{"a", "b"})

	// Too few arguments.
	if err := tb.AddRowItems("only-one"); err == nil {
		t.Error("expected ErrColumnCount for too few args, got nil")
	}

	// Too many arguments.
	if err := tb.AddRowItems("x", "y", "z"); err == nil {
		t.Error("expected ErrColumnCount for too many args, got nil")
	}

	// Table must be unchanged.
	if tb.Len() != 0 {
		t.Errorf("expected 0 rows after failed adds, got %d", tb.Len())
	}
}

// ---------------------------------------------------------------------------
// AddRow — Unicode rune counting
// ---------------------------------------------------------------------------

func TestTable_AddRow_Unicode(t *testing.T) {
	// The heading "x" has 1 rune. After adding a row with a multi-byte
	// string, maxWidth must reflect the rune count, not the byte count.
	tb, _ := New([]string{"x"})

	// "Ąnswer" — 'Ą' is a 2-byte UTF-8 sequence; the string has 6 runes.
	if err := tb.AddRow([]string{"Ąnswer"}); err != nil {
		t.Fatalf("AddRow with unicode: %v", err)
	}

	if tb.maxWidth[0] != 6 {
		t.Errorf("maxWidth[0] = %d after adding 6-rune unicode string, want 6", tb.maxWidth[0])
	}
}

// ---------------------------------------------------------------------------
// SortRows — descending order (direct call, not via SetOrderBy)
// ---------------------------------------------------------------------------

func TestTable_SortRows_Descending(t *testing.T) {
	tb, _ := New([]string{"n"})
	tb.SetPagination(0, 0)
	_ = tb.AddRow([]string{"2"})
	_ = tb.AddRow([]string{"10"})
	_ = tb.AddRow([]string{"9"})

	if err := tb.SortRows(0, false); err != nil {
		t.Fatalf("SortRows descending: %v", err)
	}

	expected := [][]string{{"10"}, {"9"}, {"2"}}
	if !reflect.DeepEqual(tb.rows, expected) {
		t.Errorf("descending numeric sort: got %v, want %v", tb.rows, expected)
	}
}

// ---------------------------------------------------------------------------
// SetStartingRow — direct tests
// ---------------------------------------------------------------------------

func TestTable_SetStartingRow_Valid(t *testing.T) {
	tb, _ := New([]string{"v"})
	tb.SetPagination(0, 0)
	_ = tb.AddRow([]string{"first"})
	_ = tb.AddRow([]string{"second"})

	// SetStartingRow(2) means start at the second data row (1-based), so
	// startingRow is stored as 1 (zero-based).
	if err := tb.SetStartingRow(2); err != nil {
		t.Fatalf("SetStartingRow(2) unexpected error: %v", err)
	}

	if tb.startingRow != 1 {
		t.Errorf("startingRow = %d, want 1", tb.startingRow)
	}

	// Only the second data row should appear in output.
	lines := tb.FormatText()
	found := false

	for _, l := range lines {
		if l != "" && contains(l, "second") {
			found = true
		}

		if l != "" && contains(l, "first") {
			t.Errorf("FormatText should skip row 0 with startingRow=2, but found 'first'")
		}
	}

	if !found {
		t.Errorf("FormatText missing 'second' with startingRow=2: %v", lines)
	}
}

func TestTable_SetStartingRow_Invalid(t *testing.T) {
	tb, _ := New([]string{"v"})

	// 0 and negative values are invalid (must be >= 1).
	for _, s := range []int{0, -1, -100} {
		if err := tb.SetStartingRow(s); err == nil {
			t.Errorf("SetStartingRow(%d): expected ErrInvalidRowNumber, got nil", s)
		}
	}
}

// contains is a simple substring helper used by these tests.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}

	return false
}

// ---------------------------------------------------------------------------
// SetMinimumWidth — success path
// ---------------------------------------------------------------------------

func TestTable_SetMinimumWidth_Valid(t *testing.T) {
	tb, _ := New([]string{"a", "b"})
	tb.SetPagination(0, 0)

	// Column 1 ("b") starts with maxWidth=1. Set minimum to 10.
	if err := tb.SetMinimumWidth(1, 10); err != nil {
		t.Fatalf("SetMinimumWidth(1, 10): %v", err)
	}

	if tb.maxWidth[1] != 10 {
		t.Errorf("maxWidth[1] = %d, want 10", tb.maxWidth[1])
	}

	// Setting a width smaller than the current maximum is a no-op.
	if err := tb.SetMinimumWidth(1, 2); err != nil {
		t.Fatalf("SetMinimumWidth(1, 2): %v", err)
	}

	if tb.maxWidth[1] != 10 {
		t.Errorf("maxWidth[1] = %d after smaller SetMinimumWidth, want 10 (unchanged)", tb.maxWidth[1])
	}
}

// ---------------------------------------------------------------------------
// SetOrderBy — empty string bug documentation
// ---------------------------------------------------------------------------

// Previously, SetOrderBy("") panicked with an index-out-of-range error
// because name[0] was accessed before checking the string length. A length
// guard now returns ErrInvalidColumnName for empty input.
func TestTable_SetOrderBy_EmptyString_ReturnsError(t *testing.T) {
	tb, _ := New([]string{"col"})

	err := tb.SetOrderBy("")
	if err == nil {
		t.Fatal("SetOrderBy(\"\") expected ErrInvalidColumnName, got nil")
	}

	if !errors.Equal(err, errors.ErrInvalidColumnName) {
		t.Errorf("SetOrderBy(\"\") error = %v, want ErrInvalidColumnName", err)
	}
}
