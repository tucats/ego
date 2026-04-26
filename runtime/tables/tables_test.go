package tables

// Unit tests for the runtime/tables package.
//
// These tests exercise every public-facing function (newTable, addRow,
// addColumn, addColumns, sortTable, closeTable, lenTable, widthTable,
// getTableElement, getRow, setPagination, setFormat, setAlignment,
// toString, printTable, and the exported Pad helper).
//
// # Test infrastructure
//
// Most table methods are called as Ego method receivers, meaning the Ego VM
// stores the table object in the symbol table under the special variable name
// defs.ThisVariable before invoking the method.  The helpers makeTableSymbols
// and addTestRow reproduce that setup so the underlying Go functions can be
// called directly in tests without going through the Ego compiler or bytecode
// engine.

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// makeTableSymbols creates a fresh root symbol table that contains a fully
// initialized Table object stored as the "this" receiver variable.  The
// returned symbol table can be passed directly to any of the runtime/tables
// method functions.
//
// headings are the column names to create the table with.  Alignment
// hints (":name", "name:", ":name:") are supported just as they are in
// production code.
func makeTableSymbols(t *testing.T, headings ...string) *symbols.SymbolTable {
	t.Helper()

	s := symbols.NewRootSymbolTable("test")

	args := make([]any, len(headings))
	for i, h := range headings {
		args[i] = h
	}

	tableStruct, err := newTable(s, data.NewList(args...))
	if err != nil {
		t.Fatalf("makeTableSymbols: newTable failed: %v", err)
	}

	s.SetAlways(defs.ThisVariable, tableStruct)

	return s
}

// addTestRow is a convenience wrapper that adds a single row of values to
// the table stored in s, failing the test immediately if the operation
// returns an error.
func addTestRow(t *testing.T, s *symbols.SymbolTable, values ...any) {
	t.Helper()

	_, err := addRow(s, data.NewList(values...))
	if err != nil {
		t.Fatalf("addTestRow: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Pad (exported utility)
// ---------------------------------------------------------------------------

func TestPad(t *testing.T) {
	tests := []struct {
		name  string
		value any
		width int
		want  string
	}{
		// Positive width → left-aligned (value on left, spaces on right).
		{
			name:  "left-align shorter string",
			value: "hi",
			width: 5,
			want:  "hi   ",
		},
		{
			name:  "left-align exact width",
			value: "hello",
			width: 5,
			want:  "hello",
		},
		{
			name:  "left-align truncates to width",
			value: "toolong",
			width: 5,
			want:  "toolon"[:5], // "toolon"[0:5] = "toolo"
		},
		// Negative width → right-aligned (spaces on left, value on right).
		{
			name:  "right-align shorter string",
			value: "hi",
			width: -5,
			want:  "   hi",
		},
		{
			name:  "right-align exact width",
			value: "hello",
			width: -5,
			want:  "hello",
		},
		{
			name:  "right-align truncates to abs(width)",
			value: "toolong",
			width: -5,
			want:  "toolon"[:5], // same truncation as left-align
		},
		// Non-string value is formatted via data.FormatUnquoted.
		{
			name:  "integer value left-aligned",
			value: 42,
			width: 6,
			want:  "42    ",
		},
		{
			name:  "integer value right-aligned",
			value: 42,
			width: -6,
			want:  "    42",
		},
		// Zero width truncates everything to empty string.
		{
			name:  "zero width returns empty string",
			value: "hi",
			width: 0,
			want:  "",
		},
		// Boolean value.
		{
			name:  "bool value",
			value: true,
			width: 6,
			want:  "true  ",
		},
		// Unicode: each of these characters is a single rune but multiple bytes.
		// Padding and truncation must count runes, not bytes.
		{
			name:  "multi-byte runes left-aligned with padding",
			value: "caf\u00e9", // "café" — 4 runes, 5 bytes
			width: 7,
			want:  "caf\u00e9   ", // 4 runes + 3 spaces = 7 runes
		},
		{
			name:  "multi-byte runes truncated at rune boundary",
			value: "caf\u00e9x", // "caféx" — 5 runes
			width: 4,
			want:  "caf\u00e9", // first 4 runes, no half-rune slice
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Pad(tt.value, tt.width)
			if got != tt.want {
				t.Errorf("Pad(%v, %d) = %q, want %q", tt.value, tt.width, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// newTable
// ---------------------------------------------------------------------------

func TestNewTable(t *testing.T) {
	t.Run("simple column names", func(t *testing.T) {
		s := symbols.NewRootSymbolTable("test")

		got, err := newTable(s, data.NewList("Name", "Age", "City"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tbl, ok := got.(*data.Struct)
		if !ok {
			t.Fatal("result is not a *data.Struct")
		}

		// Verify the Headings field is populated.
		headingsVal, found := tbl.Get(headingsFieldName)
		if !found {
			t.Fatal("Headings field not found in struct")
		}

		headingsArr, ok := headingsVal.(*data.Array)
		if !ok {
			t.Fatal("Headings field is not a *data.Array")
		}

		if headingsArr.Len() != 3 {
			t.Errorf("expected 3 headings, got %d", headingsArr.Len())
		}
	})

	t.Run("left-alignment hint (leading colon)", func(t *testing.T) {
		s := symbols.NewRootSymbolTable("test")

		got, err := newTable(s, data.NewList(":Name", "Age"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tbl := got.(*data.Struct)
		headingsArr := tbl.GetAlways(headingsFieldName).(*data.Array)

		// The colon should be stripped from the heading name.
		v, _ := headingsArr.Get(0)
		if data.String(v) != "Name" {
			t.Errorf("expected heading %q, got %q", "Name", data.String(v))
		}
	})

	t.Run("right-alignment hint (trailing colon)", func(t *testing.T) {
		s := symbols.NewRootSymbolTable("test")

		got, err := newTable(s, data.NewList("Score:", "Name"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tbl := got.(*data.Struct)
		headingsArr := tbl.GetAlways(headingsFieldName).(*data.Array)

		v, _ := headingsArr.Get(0)
		if data.String(v) != "Score" {
			t.Errorf("expected heading %q, got %q", "Score", data.String(v))
		}
	})

	t.Run("center-alignment hint (both colons)", func(t *testing.T) {
		s := symbols.NewRootSymbolTable("test")

		got, err := newTable(s, data.NewList(":Label:"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tbl := got.(*data.Struct)
		headingsArr := tbl.GetAlways(headingsFieldName).(*data.Array)

		v, _ := headingsArr.Get(0)
		if data.String(v) != "Label" {
			t.Errorf("expected heading %q, got %q", "Label", data.String(v))
		}
	})

	t.Run("headings passed as *data.Array", func(t *testing.T) {
		s := symbols.NewRootSymbolTable("test")

		arr := data.NewArray(data.StringType, 2)
		_ = arr.Set(0, "A")
		_ = arr.Set(1, "B")

		got, err := newTable(s, data.NewList(arr))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tbl := got.(*data.Struct)
		headingsArr := tbl.GetAlways(headingsFieldName).(*data.Array)

		if headingsArr.Len() != 2 {
			t.Errorf("expected 2 headings, got %d", headingsArr.Len())
		}
	})

	t.Run("no headings creates empty table", func(t *testing.T) {
		s := symbols.NewRootSymbolTable("test")

		got, err := newTable(s, data.NewList())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got == nil {
			t.Fatal("expected non-nil result")
		}
	})
}

// ---------------------------------------------------------------------------
// lenTable and widthTable
// ---------------------------------------------------------------------------

func TestLenTable(t *testing.T) {
	t.Run("empty table has zero rows", func(t *testing.T) {
		s := makeTableSymbols(t, "A", "B")

		got, err := lenTable(s, data.NewList())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != 0 {
			t.Errorf("expected 0, got %v", got)
		}
	})

	t.Run("len grows with each added row", func(t *testing.T) {
		s := makeTableSymbols(t, "A", "B")

		addTestRow(t, s, "x", "y")
		addTestRow(t, s, "p", "q")

		got, err := lenTable(s, data.NewList())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != 2 {
			t.Errorf("expected 2, got %v", got)
		}
	})

	t.Run("no receiver returns error", func(t *testing.T) {
		s := symbols.NewRootSymbolTable("test")

		_, err := lenTable(s, data.NewList())
		if err == nil {
			t.Error("expected error when no receiver present")
		}
	})
}

func TestWidthTable(t *testing.T) {
	t.Run("width matches column count at creation", func(t *testing.T) {
		s := makeTableSymbols(t, "Name", "Age", "City")

		got, err := widthTable(s, data.NewList())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != 3 {
			t.Errorf("expected 3, got %v", got)
		}
	})

	t.Run("width increases after AddColumn", func(t *testing.T) {
		s := makeTableSymbols(t, "A", "B")

		_, _ = addColumn(s, data.NewList("C"))

		got, err := widthTable(s, data.NewList())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != 3 {
			t.Errorf("expected 3, got %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// addRow
// ---------------------------------------------------------------------------

func TestAddRow(t *testing.T) {
	t.Run("variadic string values", func(t *testing.T) {
		s := makeTableSymbols(t, "Name", "Score")

		_, err := addRow(s, data.NewList("Alice", "95"))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		got, _ := lenTable(s, data.NewList())
		if got != 1 {
			t.Errorf("expected 1 row, got %v", got)
		}
	})

	t.Run("integer values are coerced to strings", func(t *testing.T) {
		s := makeTableSymbols(t, "Name", "Score")

		_, err := addRow(s, data.NewList("Bob", 88))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("wrong column count returns error", func(t *testing.T) {
		s := makeTableSymbols(t, "A", "B")

		_, err := addRow(s, data.NewList("only-one"))
		if err == nil {
			t.Error("expected error for mismatched column count")
		}
	})

	t.Run("no arguments returns error", func(t *testing.T) {
		s := makeTableSymbols(t, "A", "B")

		_, err := addRow(s, data.NewList())
		if err == nil {
			t.Error("expected error for empty argument list")
		}
	})

	t.Run("[]any argument adds row", func(t *testing.T) {
		s := makeTableSymbols(t, "Name", "Score")

		_, err := addRow(s, data.NewList([]any{"Carol", "77"}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("struct argument maps fields to columns", func(t *testing.T) {
		s := makeTableSymbols(t, "Name", "Score")

		// Build a *data.Struct with matching field names.
		structType := data.StructureType().
			DefineField("Name", data.StringType).
			DefineField("Score", data.StringType)

		row := data.NewStruct(data.TypeDefinition("Row", structType))
		row.SetAlways("Name", "Diana")
		row.SetAlways("Score", "91")

		_, err := addRow(s, data.NewList(row))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		got, _ := lenTable(s, data.NewList())
		if got != 1 {
			t.Errorf("expected 1 row, got %v", got)
		}
	})
}

// ---------------------------------------------------------------------------
// addColumn / addColumns
// ---------------------------------------------------------------------------

func TestAddColumn(t *testing.T) {
	t.Run("adds column and updates Headings array", func(t *testing.T) {
		s := makeTableSymbols(t, "A")

		_, err := addColumn(s, data.NewList("B"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := widthTable(s, data.NewList())
		if got != 2 {
			t.Errorf("expected width 2, got %v", got)
		}

		// Verify the Headings array also reflects the new column.
		this := getThisStruct(s)
		arr := this.GetAlways(headingsFieldName).(*data.Array)

		if arr.Len() != 2 {
			t.Errorf("expected 2 headings, got %d", arr.Len())
		}
	})

	t.Run("empty heading name returns error", func(t *testing.T) {
		s := makeTableSymbols(t, "A")

		_, err := addColumn(s, data.NewList(""))
		if err == nil {
			t.Error("expected error for empty heading")
		}
	})
}

func TestAddColumns(t *testing.T) {
	t.Run("adds multiple columns in one call", func(t *testing.T) {
		s := makeTableSymbols(t, "A")

		_, err := addColumns(s, data.NewList("B", "C", "D"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := widthTable(s, data.NewList())
		if got != 4 {
			t.Errorf("expected width 4, got %v", got)
		}
	})

	t.Run("stops on first invalid heading", func(t *testing.T) {
		s := makeTableSymbols(t, "A")

		_, err := addColumns(s, data.NewList("B", "", "C"))
		if err == nil {
			t.Error("expected error for empty heading in list")
		}
	})
}

// ---------------------------------------------------------------------------
// sortTable
// ---------------------------------------------------------------------------

func TestSortTable(t *testing.T) {
	const favoriteAunt = "Alice"

	t.Run("ascending sort by first column", func(t *testing.T) {
		s := makeTableSymbols(t, "Name", "Score")

		addTestRow(t, s, "Charlie", "80")
		addTestRow(t, s, favoriteAunt, "90")
		addTestRow(t, s, "Bob", "85")

		_, err := sortTable(s, data.NewList("Name"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// First row should now be Alice.
		result, err := getTableElement(s, data.NewList(0, "Name"))
		if err != nil {
			t.Fatalf("unexpected error reading sorted row: %v", err)
		}

		list := result.(data.List)
		if data.String(list.Get(0)) != favoriteAunt {
			t.Errorf("expected first row to be Alice, got %q", data.String(list.Get(0)))
		}
	})

	t.Run("descending sort with tilde prefix", func(t *testing.T) {
		s := makeTableSymbols(t, "Name", "Score")

		addTestRow(t, s, "Alice", "90")
		addTestRow(t, s, "Bob", "85")
		addTestRow(t, s, "Charlie", "80")

		_, err := sortTable(s, data.NewList("~Name"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, _ := getTableElement(s, data.NewList(0, "Name"))
		list := result.(data.List)

		if data.String(list.Get(0)) != "Charlie" {
			t.Errorf("expected first row to be Charlie after descending sort, got %q", data.String(list.Get(0)))
		}
	})

	t.Run("multi-key sort (least significant first)", func(t *testing.T) {
		s := makeTableSymbols(t, "Group", "Name")

		addTestRow(t, s, "B", "Zed")
		addTestRow(t, s, "A", "Zed")
		addTestRow(t, s, "A", "Alice")

		// Sort by Name then Group (Group is primary key).
		_, err := sortTable(s, data.NewList("Group", "Name"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// After sort: (A, Alice), (A, Zed), (B, Zed)
		r0, _ := getTableElement(s, data.NewList(0, "Name"))
		name0 := data.String(r0.(data.List).Get(0))

		if name0 != "Alice" {
			t.Errorf("expected first row Name=Alice, got %q", name0)
		}
	})

	t.Run("invalid column name returns error", func(t *testing.T) {
		s := makeTableSymbols(t, "Name")

		_, err := sortTable(s, data.NewList("NoSuchColumn"))
		if err == nil {
			t.Error("expected error for unknown column name")
		}
	})
}

// ---------------------------------------------------------------------------
// getTableElement (Get)
// ---------------------------------------------------------------------------

func TestGetTableElement(t *testing.T) {
	t.Run("retrieves correct cell value", func(t *testing.T) {
		s := makeTableSymbols(t, "Name", "Score")

		addTestRow(t, s, "Alice", "95")
		addTestRow(t, s, "Bob", "80")

		result, err := getTableElement(s, data.NewList(1, "Score"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		list := result.(data.List)
		if data.String(list.Get(0)) != "80" {
			t.Errorf("expected %q, got %q", "80", data.String(list.Get(0)))
		}
	})

	t.Run("column lookup is case-insensitive", func(t *testing.T) {
		s := makeTableSymbols(t, "Name", "Score")

		addTestRow(t, s, "Alice", "95")

		result, err := getTableElement(s, data.NewList(0, "score")) // lowercase
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		list := result.(data.List)
		if data.String(list.Get(0)) != "95" {
			t.Errorf("expected %q, got %q", "95", data.String(list.Get(0)))
		}
	})

	t.Run("negative row index returns error", func(t *testing.T) {
		s := makeTableSymbols(t, "Name")

		addTestRow(t, s, "Alice")

		_, err := getTableElement(s, data.NewList(-1, "Name"))
		if err == nil {
			t.Error("expected error for negative row index")
		}
	})

	t.Run("out-of-bounds row index returns error", func(t *testing.T) {
		s := makeTableSymbols(t, "Name")

		addTestRow(t, s, "Alice")

		_, err := getTableElement(s, data.NewList(5, "Name"))
		if err == nil {
			t.Error("expected error for out-of-bounds row index")
		}
	})

	t.Run("unknown column name returns error", func(t *testing.T) {
		s := makeTableSymbols(t, "Name")

		addTestRow(t, s, "Alice")

		_, err := getTableElement(s, data.NewList(0, "NoSuchCol"))
		if err == nil {
			t.Error("expected error for unknown column name")
		}
	})
}

// ---------------------------------------------------------------------------
// getRow (GetRow)
// ---------------------------------------------------------------------------

func TestGetRow(t *testing.T) {
	t.Run("returns all column values for a row", func(t *testing.T) {
		s := makeTableSymbols(t, "Name", "Score", "City")

		addTestRow(t, s, "Alice", "95", "Springfield")

		result, err := getRow(s, data.NewList(0))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		list := result.(data.List)
		arr, ok := list.Get(0).(*data.Array)

		if !ok {
			t.Fatalf("expected *data.Array, got %T", list.Get(0))
		}

		if arr.Len() != 3 {
			t.Errorf("expected 3 elements, got %d", arr.Len())
		}

		v, _ := arr.Get(0)
		if data.String(v) != "Alice" {
			t.Errorf("expected Alice, got %q", data.String(v))
		}
	})

	t.Run("negative row index returns error", func(t *testing.T) {
		s := makeTableSymbols(t, "Name")

		addTestRow(t, s, "Alice")

		_, err := getRow(s, data.NewList(-1))
		if err == nil {
			t.Error("expected error for negative row index")
		}
	})

	t.Run("out-of-bounds index returns error", func(t *testing.T) {
		s := makeTableSymbols(t, "Name")

		addTestRow(t, s, "Alice")

		_, err := getRow(s, data.NewList(99))
		if err == nil {
			t.Error("expected error for out-of-bounds index")
		}
	})
}

// ---------------------------------------------------------------------------
// closeTable
// ---------------------------------------------------------------------------

func TestCloseTable(t *testing.T) {
	t.Run("close sets table field to nil", func(t *testing.T) {
		s := makeTableSymbols(t, "A", "B")

		got, err := closeTable(s, data.NewList())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != true {
			t.Errorf("expected true return value, got %v", got)
		}

		// After close, getTable should return an error.
		_, err = getTable(s)
		if err == nil {
			t.Error("expected error when accessing closed table")
		}
	})

	t.Run("calling method on closed table returns error", func(t *testing.T) {
		s := makeTableSymbols(t, "A")

		_, _ = closeTable(s, data.NewList())

		_, err := lenTable(s, data.NewList())
		if err == nil {
			t.Error("expected error when calling Len on closed table")
		}
	})
}

// ---------------------------------------------------------------------------
// setPagination
// ---------------------------------------------------------------------------

func TestSetPagination(t *testing.T) {
	t.Run("sets pagination without error", func(t *testing.T) {
		s := makeTableSymbols(t, "Name")

		// args: height=40 rows, width=120 columns — matches the declaration.
		got, err := setPagination(s, data.NewList(40, 120))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if got != true {
			t.Errorf("expected true, got %v", got)
		}
	})

	t.Run("non-integer first arg returns error", func(t *testing.T) {
		s := makeTableSymbols(t, "Name")

		_, err := setPagination(s, data.NewList("not-a-number", 80))
		if err == nil {
			t.Error("expected error for non-integer height")
		}
	})

	t.Run("non-integer second arg returns error", func(t *testing.T) {
		s := makeTableSymbols(t, "Name")

		_, err := setPagination(s, data.NewList(40, "not-a-number"))
		if err == nil {
			t.Error("expected error for non-integer width")
		}
	})
}

// ---------------------------------------------------------------------------
// setFormat
// ---------------------------------------------------------------------------

func TestSetFormat(t *testing.T) {
	t.Run("no args defaults to headings and underlines on", func(t *testing.T) {
		s := makeTableSymbols(t, "Name")

		_, err := setFormat(s, data.NewList())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("single false arg turns off both headings and underlines", func(t *testing.T) {
		s := makeTableSymbols(t, "Name")

		_, err := setFormat(s, data.NewList(false))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("two args control headings and underlines independently", func(t *testing.T) {
		s := makeTableSymbols(t, "Name")

		_, err := setFormat(s, data.NewList(true, false))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// setAlignment
// ---------------------------------------------------------------------------

func TestSetAlignment(t *testing.T) {
	tests := []struct {
		name      string
		column    any // string name or int index
		alignment string
		wantErr   bool
	}{
		{"left by name", "Score", "left", false},
		{"right by name", "Score", "right", false},
		{"center by name", "Score", "center", false},
		{"case-insensitive alignment", "Score", "CENTER", false},
		{"by integer index", 1, "right", false},
		{"invalid alignment string", "Score", "diagonal", true},
		{"unknown column name", "NoSuchCol", "left", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := makeTableSymbols(t, "Name", "Score")

			_, err := setAlignment(s, data.NewList(tt.column, tt.alignment))
			if (err != nil) != tt.wantErr {
				t.Errorf("setAlignment(%v, %q) error = %v, wantErr %v", tt.column, tt.alignment, err, tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// toString
// ---------------------------------------------------------------------------

func TestToString(t *testing.T) {
	t.Run("text format contains column names and data", func(t *testing.T) {
		s := makeTableSymbols(t, "Name", "Score")

		addTestRow(t, s, "Alice", "95")
		addTestRow(t, s, "Bob", "80")

		got, err := toString(s, data.NewList("text"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		str, ok := got.(string)
		if !ok {
			t.Fatalf("expected string, got %T", got)
		}

		for _, want := range []string{"Name", "Score", "Alice", "95", "Bob", "80"} {
			if !strings.Contains(str, want) {
				t.Errorf("expected output to contain %q, got:\n%s", want, str)
			}
		}
	})

	t.Run("json format is valid JSON array", func(t *testing.T) {
		s := makeTableSymbols(t, "Name", "Score")

		addTestRow(t, s, "Alice", "95")

		got, err := toString(s, data.NewList("json"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		str := data.String(got)
		if !strings.HasPrefix(str, "[") || !strings.HasSuffix(strings.TrimSpace(str), "]") {
			t.Errorf("expected JSON array, got: %q", str)
		}
	})

	t.Run("no-format arg uses default format", func(t *testing.T) {
		s := makeTableSymbols(t, "X")

		addTestRow(t, s, "val")

		_, err := toString(s, data.NewList())
		if err != nil {
			t.Errorf("unexpected error with no format arg: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// printTable (with custom writer)
// ---------------------------------------------------------------------------

func TestPrintTable(t *testing.T) {
	t.Run("writes to custom io.Writer when symbol is present", func(t *testing.T) {
		s := makeTableSymbols(t, "Name", "Score")

		addTestRow(t, s, "Alice", "95")

		var buf bytes.Buffer
		
		s.SetAlways(defs.StdoutWriterSymbol, &buf)

		_, err := printTable(s, data.NewList("text"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(buf.String(), "Alice") {
			t.Errorf("expected output to contain Alice, got: %q", buf.String())
		}
	})
}

// ---------------------------------------------------------------------------
// getTable and getThisStruct helpers
// ---------------------------------------------------------------------------

func TestGetTable(t *testing.T) {
	t.Run("returns error when no receiver", func(t *testing.T) {
		s := symbols.NewRootSymbolTable("test")

		_, err := getTable(s)
		if err == nil {
			t.Error("expected error when ThisVariable not set")
		}
	})

	t.Run("returns error when receiver is not a struct", func(t *testing.T) {
		s := symbols.NewRootSymbolTable("test")
		s.SetAlways(defs.ThisVariable, 42) // not a struct

		_, err := getTable(s)
		if err == nil {
			t.Error("expected error when receiver is not a struct")
		}
	})

	t.Run("returns table when receiver is valid", func(t *testing.T) {
		s := makeTableSymbols(t, "A", "B")

		tbl, err := getTable(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if tbl == nil {
			t.Error("expected non-nil table")
		}
	})
}

func TestGetThisStruct(t *testing.T) {
	t.Run("returns nil when no receiver", func(t *testing.T) {
		s := symbols.NewRootSymbolTable("test")

		got := getThisStruct(s)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("returns nil when receiver is not a struct", func(t *testing.T) {
		s := symbols.NewRootSymbolTable("test")
		s.SetAlways(defs.ThisVariable, "not-a-struct")

		got := getThisStruct(s)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("returns struct when receiver is valid", func(t *testing.T) {
		s := makeTableSymbols(t, "A")

		got := getThisStruct(s)
		if got == nil {
			t.Error("expected non-nil struct")
		}
	})
}

// ---------------------------------------------------------------------------
// Round-trip: create, populate, sort, read
// ---------------------------------------------------------------------------

func TestRoundTrip(t *testing.T) {
	// Build a table, add rows in unsorted order, sort it, then verify the
	// full row contents via GetRow.
	s := makeTableSymbols(t, "Name", "Score", "City")

	addTestRow(t, s, "Charlie", "80", "Denver")
	addTestRow(t, s, "Alice", "95", "Boston")
	addTestRow(t, s, "Bob", "85", "Austin")

	_, err := sortTable(s, data.NewList("Name"))
	if err != nil {
		t.Fatalf("sort failed: %v", err)
	}

	type row struct{ name, score, city string }

	expected := []row{
		{"Alice", "95", "Boston"},
		{"Bob", "85", "Austin"},
		{"Charlie", "80", "Denver"},
	}

	for i, want := range expected {
		result, err := getRow(s, data.NewList(i))
		if err != nil {
			t.Fatalf("getRow(%d): %v", i, err)
		}

		list := result.(data.List)
		arr := list.Get(0).(*data.Array)

		nameVal, _ := arr.Get(0)
		scoreVal, _ := arr.Get(1)
		cityVal, _ := arr.Get(2)

		got := row{data.String(nameVal), data.String(scoreVal), data.String(cityVal)}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("row[%d]: got %+v, want %+v", i, got, want)
		}
	}
}
