package tables

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tucats/ego/app-cli/ui"
)

// ---------------------------------------------------------------------------
// Table dimensions: Len, Width
// ---------------------------------------------------------------------------

func TestTable_Len(t *testing.T) {
	tb, _ := New([]string{"a", "b"})

	if tb.Len() != 0 {
		t.Fatalf("Len() = %d, want 0 on empty table", tb.Len())
	}

	_ = tb.AddRow([]string{"x", "y"})

	if tb.Len() != 1 {
		t.Fatalf("Len() = %d, want 1 after one AddRow", tb.Len())
	}

	_ = tb.AddRow([]string{"p", "q"})

	if tb.Len() != 2 {
		t.Fatalf("Len() = %d, want 2 after two AddRow calls", tb.Len())
	}
}

func TestTable_Width(t *testing.T) {
	tb, _ := New([]string{"x", "y", "z"})
	if tb.Width() != 3 {
		t.Fatalf("Width() = %d, want 3", tb.Width())
	}
}

// ---------------------------------------------------------------------------
// Column lookup and heading accessors
// ---------------------------------------------------------------------------

func TestTable_Column(t *testing.T) {
	tb, _ := New([]string{"Name", "Age", "City"})

	tests := []struct {
		name      string
		colName   string
		wantIndex int
		wantFound bool
	}{
		{"exact match first column", "Name", 0, true},
		{"exact match last column", "City", 2, true},
		{"case-insensitive match", "AGE", 1, true},
		{"not found", "Score", -1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := tb.Column(tt.colName)
			if found != tt.wantFound || got != tt.wantIndex {
				t.Errorf("Column(%q) = (%d, %v), want (%d, %v)",
					tt.colName, got, found, tt.wantIndex, tt.wantFound)
			}
		})
	}
}

func TestTable_GetHeadings(t *testing.T) {
	headings := []string{"Alpha", "Beta", "Gamma"}
	tb, _ := New(headings)

	got := tb.GetHeadings()
	if !reflect.DeepEqual(got, headings) {
		t.Errorf("GetHeadings() = %v, want %v", got, headings)
	}
}

// ---------------------------------------------------------------------------
// AddColumn / AddColumns
// ---------------------------------------------------------------------------

func TestTable_AddColumn(t *testing.T) {
	t.Run("add column to empty table", func(t *testing.T) {
		tb, _ := New([]string{"first"})
		if err := tb.AddColumn("second"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tb.Width() != 2 {
			t.Errorf("Width() = %d, want 2", tb.Width())
		}

		if tb.names[1] != "second" {
			t.Errorf("names[1] = %q, want %q", tb.names[1], "second")
		}
	})

	t.Run("new column extends existing rows", func(t *testing.T) {
		tb, _ := New([]string{"a"})
		_ = tb.AddRow([]string{"v1"})
		_ = tb.AddRow([]string{"v2"})

		if err := tb.AddColumn("b"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Both existing rows should now have an empty string in the new column.
		for i, row := range tb.rows {
			if len(row) != 2 {
				t.Errorf("row %d has %d columns, want 2", i, len(row))
			}

			if row[1] != "" {
				t.Errorf("row %d col 1 = %q, want empty string", i, row[1])
			}
		}
	})

	t.Run("empty column name is rejected", func(t *testing.T) {
		tb, _ := New([]string{"first"})
		if err := tb.AddColumn(""); err == nil {
			t.Error("expected error for empty column name, got nil")
		}
	})
}

func TestTable_AddColumns(t *testing.T) {
	t.Run("add multiple columns", func(t *testing.T) {
		tb, _ := New([]string{"id"})
		if err := tb.AddColumns("name", "score"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tb.Width() != 3 {
			t.Errorf("Width() = %d, want 3", tb.Width())
		}
	})

	t.Run("empty list is a no-op", func(t *testing.T) {
		tb, _ := New([]string{"id"})
		if err := tb.AddColumns(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tb.Width() != 1 {
			t.Errorf("Width() = %d, want 1", tb.Width())
		}
	})

	t.Run("duplicate column name is rejected", func(t *testing.T) {
		tb, _ := New([]string{"id"})
		if err := tb.AddColumns("name", "id"); err == nil {
			t.Error("expected error for duplicate column name, got nil")
		}
	})

	t.Run("empty column name inside list is rejected", func(t *testing.T) {
		tb, _ := New([]string{"id"})
		if err := tb.AddColumns("ok", ""); err == nil {
			t.Error("expected error for empty column name, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// SetColumnOrder / SetColumnOrderByName
// ---------------------------------------------------------------------------

func TestTable_SetColumnOrder(t *testing.T) {
	tb, _ := New([]string{"a", "b", "c"})

	tests := []struct {
		name    string
		order   []int
		wantErr bool
	}{
		{"valid reorder", []int{3, 1, 2}, false},
		{"empty list is rejected", []int{}, true},
		{"column number too high", []int{1, 4}, true},
		{"column number too low (0)", []int{0, 1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tb.SetColumnOrder(tt.order)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetColumnOrder(%v) error = %v, wantErr %v", tt.order, err, tt.wantErr)
			}
		})
	}
}

func TestTable_SetColumnOrderByName(t *testing.T) {
	tb, _ := New([]string{"name", "age", "city"})

	tests := []struct {
		name    string
		order   []string
		wantErr bool
	}{
		{"valid name order", []string{"city", "name", "age"}, false},
		{"empty list is rejected", []string{}, true},
		{"unknown column name", []string{"name", "score"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tb.SetColumnOrderByName(tt.order)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetColumnOrderByName(%v) error = %v, wantErr %v", tt.order, err, tt.wantErr)
			}
		})
	}
}

// Verify that SetColumnOrderByName actually reorders output columns.
func TestTable_SetColumnOrderByName_ReordersOutput(t *testing.T) {
	tb, _ := New([]string{"first", "second"})
	tb.SetPagination(0, 0)
	tb.ShowHeadings(false)
	_ = tb.AddRow([]string{"A", "B"})

	// Reverse the column order so "second" prints before "first".
	if err := tb.SetColumnOrderByName([]string{"second", "first"}); err != nil {
		t.Fatalf("SetColumnOrderByName error: %v", err)
	}

	lines := tb.FormatText()
	if len(lines) == 0 {
		t.Fatal("FormatText returned no lines")
	}

	// With default spacing the line should start with "B" (the second column value).
	if !strings.HasPrefix(strings.TrimSpace(lines[0]), "B") {
		t.Errorf("expected line to start with 'B' after column reorder, got %q", lines[0])
	}
}

// ---------------------------------------------------------------------------
// GetRow
// ---------------------------------------------------------------------------

func TestTable_GetRow(t *testing.T) {
	tb, _ := New([]string{"x", "y"})
	_ = tb.AddRow([]string{"hello", "world"})

	t.Run("valid index", func(t *testing.T) {
		row, err := tb.GetRow(0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(row, []string{"hello", "world"}) {
			t.Errorf("GetRow(0) = %v, want [hello world]", row)
		}
	})

	t.Run("negative index", func(t *testing.T) {
		if _, err := tb.GetRow(-1); err == nil {
			t.Error("expected error for negative index, got nil")
		}
	})

	t.Run("index out of range", func(t *testing.T) {
		if _, err := tb.GetRow(5); err == nil {
			t.Error("expected error for out-of-range index, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// Fluent setters: RowLimit, ShowUnderlines, ShowHeadings,
// ShowRowNumbers, SetPagination
// ---------------------------------------------------------------------------

func TestTable_RowLimit(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{5, 5},
		{0, -1},  // zero → unlimited
		{-3, -1}, // negative → unlimited
	}

	for _, tt := range tests {
		tb, _ := New([]string{"col"})

		tb.RowLimit(tt.input)

		if tb.rowLimit != tt.expected {
			t.Errorf("RowLimit(%d) → rowLimit = %d, want %d", tt.input, tb.rowLimit, tt.expected)
		}
	}
}

func TestTable_ShowUnderlines(t *testing.T) {
	tb, _ := New([]string{"col"})

	ret := tb.ShowUnderlines(false)
	if ret != tb {
		t.Error("ShowUnderlines() did not return the receiver")
	}

	if tb.showUnderlines {
		t.Error("showUnderlines should be false")
	}

	tb.ShowUnderlines(true)

	if !tb.showUnderlines {
		t.Error("showUnderlines should be true")
	}
}

func TestTable_ShowHeadings(t *testing.T) {
	tb, _ := New([]string{"col"})

	ret := tb.ShowHeadings(false)
	if ret != tb {
		t.Error("ShowHeadings() did not return the receiver")
	}

	if tb.showHeadings {
		t.Error("showHeadings should be false")
	}
}

func TestTable_ShowRowNumbers(t *testing.T) {
	tb, _ := New([]string{"col"})

	ret := tb.ShowRowNumbers(true)
	if ret != tb {
		t.Error("ShowRowNumbers() did not return the receiver")
	}

	if !tb.showRowNumbers {
		t.Error("showRowNumbers should be true")
	}
}

func TestTable_SetPagination(t *testing.T) {
	tb, _ := New([]string{"col"})

	ret := tb.SetPagination(40, 120)
	if ret != tb {
		t.Error("SetPagination() did not return the receiver")
	}

	if tb.terminalHeight != 40 || tb.terminalWidth != 120 {
		t.Errorf("SetPagination(40,120) → height=%d width=%d", tb.terminalHeight, tb.terminalWidth)
	}
}

// ---------------------------------------------------------------------------
// SetMinimumWidth error paths (success path is covered in rows_test.go)
// ---------------------------------------------------------------------------

func TestTable_SetMinimumWidth_Errors(t *testing.T) {
	tb, _ := New([]string{"a", "b"})

	if err := tb.SetMinimumWidth(-1, 10); err == nil {
		t.Error("expected error for negative column index, got nil")
	}

	if err := tb.SetMinimumWidth(5, 10); err == nil {
		t.Error("expected error for out-of-range column index, got nil")
	}

	if err := tb.SetMinimumWidth(0, -5); err == nil {
		t.Error("expected error for negative width, got nil")
	}
}

// ---------------------------------------------------------------------------
// FormatJSON with rowLimit and startingRow
// ---------------------------------------------------------------------------

func TestTable_FormatJSON_Pagination(t *testing.T) {
	tb, _ := New([]string{"v"})
	tb.SetPagination(0, 0)
	_ = tb.AddRow([]string{"one"})
	_ = tb.AddRow([]string{"two"})
	_ = tb.AddRow([]string{"three"})

	t.Run("row limit", func(t *testing.T) {
		tb.RowLimit(2)
		got := tb.FormatJSON()
		// Expect exactly two objects in the array.
		if strings.Count(got, "{") != 2 {
			t.Errorf("expected 2 objects with rowLimit=2, got: %s", got)
		}
	})

	t.Run("starting row", func(t *testing.T) {
		tb.RowLimit(0)     // unlimited
		tb.startingRow = 1 // skip first row

		got := tb.FormatJSON()
		if strings.Contains(got, `"one"`) {
			t.Errorf("FormatJSON with startingRow=1 should skip row 0, got: %s", got)
		}
	})
}

// ---------------------------------------------------------------------------
// FormatIndented
// ---------------------------------------------------------------------------

func TestTable_FormatIndented(t *testing.T) {
	tb, _ := New([]string{"name", "score"})
	tb.SetPagination(0, 0)
	_ = tb.AddRow([]string{"Alice", "42"})
	_ = tb.AddRow([]string{"Bob", "99"})

	got := tb.FormatIndented()

	if !strings.HasPrefix(got, "[\n") {
		t.Errorf("FormatIndented should start with '[\\n', got: %q", got[:10])
	}

	if !strings.Contains(got, `"name"`) {
		t.Errorf("FormatIndented missing 'name' key: %s", got)
	}

	if !strings.Contains(got, `"Alice"`) {
		t.Errorf("FormatIndented missing 'Alice': %s", got)
	}

	if !strings.Contains(got, `"score":99`) {
		t.Errorf("FormatIndented missing numeric score:99, got: %s", got)
	}
}

// ---------------------------------------------------------------------------
// String() — text and JSON variants
// ---------------------------------------------------------------------------

func TestTable_String(t *testing.T) {
	tb, _ := New([]string{"fruit"})
	tb.SetPagination(0, 0)
	_ = tb.AddRow([]string{"apple"})

	t.Run("text format", func(t *testing.T) {
		got, err := tb.String(ui.TextFormat)
		if err != nil {
			t.Fatalf("String(text) error: %v", err)
		}

		if !strings.Contains(got, "fruit") {
			t.Errorf("String(text) missing heading 'fruit': %q", got)
		}

		if !strings.Contains(got, "apple") {
			t.Errorf("String(text) missing row value 'apple': %q", got)
		}
	})

	t.Run("JSON format", func(t *testing.T) {
		got, err := tb.String(ui.JSONFormat)
		if err != nil {
			t.Fatalf("String(JSON) error: %v", err)
		}

		if !strings.Contains(got, `"fruit"`) {
			t.Errorf("String(JSON) missing key 'fruit': %q", got)
		}

		if !strings.Contains(got, `"apple"`) {
			t.Errorf("String(JSON) missing value 'apple': %q", got)
		}
	})

	t.Run("invalid format returns error", func(t *testing.T) {
		if _, err := tb.String("bogus"); err == nil {
			t.Error("expected error for invalid format, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// SortRows — numeric sort, invalid column
// ---------------------------------------------------------------------------

func TestTable_SortRows_Numeric(t *testing.T) {
	// SortRows should sort numerically when values parse as integers,
	// not lexicographically (where "9" > "10").
	tb, _ := New([]string{"n"})
	tb.SetPagination(0, 0)
	_ = tb.AddRow([]string{"10"})
	_ = tb.AddRow([]string{"9"})
	_ = tb.AddRow([]string{"2"})

	if err := tb.SortRows(0, true); err != nil {
		t.Fatalf("SortRows error: %v", err)
	}

	expected := [][]string{{"2"}, {"9"}, {"10"}}
	if !reflect.DeepEqual(tb.rows, expected) {
		t.Errorf("numeric sort got %v, want %v", tb.rows, expected)
	}
}

func TestTable_SortRows_InvalidColumn(t *testing.T) {
	tb, _ := New([]string{"a"})
	if err := tb.SortRows(-1, true); err == nil {
		t.Error("expected error for negative column, got nil")
	}

	if err := tb.SortRows(5, true); err == nil {
		t.Error("expected error for out-of-range column, got nil")
	}
}

// ---------------------------------------------------------------------------
// FormatText — row numbers and column ordering
// ---------------------------------------------------------------------------

func TestTable_FormatText_RowNumbers(t *testing.T) {
	tb, _ := New([]string{"val"})
	tb.SetPagination(0, 0)
	tb.ShowRowNumbers(true)
	_ = tb.AddRow([]string{"x"})
	_ = tb.AddRow([]string{"y"})

	lines := tb.FormatText()

	// With row numbers the data lines should contain the row number prefix.
	// Row 1 data line: "  1    x    "
	found := false

	for _, l := range lines {
		if strings.Contains(l, "1") && strings.Contains(l, "x") {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("FormatText with row numbers did not include row number in output: %v", lines)
	}
}

func TestTable_FormatText_ColumnOrder(t *testing.T) {
	tb, _ := New([]string{"first", "second"})
	tb.SetPagination(0, 0)
	tb.ShowHeadings(false)
	_ = tb.AddRow([]string{"A", "B"})

	// Print second column before first.
	_ = tb.SetColumnOrder([]int{2, 1})

	lines := tb.FormatText()
	if len(lines) == 0 {
		t.Fatal("FormatText returned no lines")
	}

	// Line should start with "B" (second column value) after reorder.
	if !strings.HasPrefix(strings.TrimSpace(lines[0]), "B") {
		t.Errorf("expected line to begin with 'B', got %q", lines[0])
	}
}
