package tables

import (
	"reflect"
	"testing"
)

// ---------------------------------------------------------------------------
// CsvSplit tests
// ---------------------------------------------------------------------------

func TestCsvSplit(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty string returns nil",
			input: "",
			want:  nil,
		},
		{
			name:  "single value no comma",
			input: "Name",
			want:  []string{"Name"},
		},
		{
			name:  "two simple values",
			input: "Name,Age",
			want:  []string{"Name", "Age"},
		},
		{
			name:  "spaces around values are trimmed",
			input: " Name , Age ",
			want:  []string{"Name", "Age"},
		},
		{
			name:  "quoted comma is not a separator",
			input: `"Name,Age",City`,
			want:  []string{"Name,Age", "City"},
		},
		{
			name:  "multiple quoted commas",
			input: `"a,b,c",d`,
			want:  []string{"a,b,c", "d"},
		},
		{
			name:  "quote characters are removed from output",
			input: `"hello"`,
			want:  []string{"hello"},
		},
		{
			name:  "three columns with spaces and quotes",
			input: `  First , "Last, Jr." , Age  `,
			want:  []string{"First", "Last, Jr.", "Age"},
		},
		// A trailing comma with no following text produces no extra element
		// because CsvSplit only appends when currentHeading.Len() > 0.
		{
			name:  "trailing comma without following text produces no extra element",
			input: "a,b,",
			want:  []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CsvSplit(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CsvSplit(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AddCSVRow tests
// ---------------------------------------------------------------------------

func TestTable_AddCSVRow(t *testing.T) {
	t.Run("valid row with matching column count", func(t *testing.T) {
		tb, err := NewCSV("Name,Age,City")
		if err != nil {
			t.Fatalf("NewCSV error: %v", err)
		}

		if err := tb.AddCSVRow("Alice,30,Boston"); err != nil {
			t.Fatalf("AddCSVRow error: %v", err)
		}

		if tb.Len() != 1 {
			t.Fatalf("expected 1 row, got %d", tb.Len())
		}

		row, _ := tb.GetRow(0)
		if !reflect.DeepEqual(row, []string{"Alice", "30", "Boston"}) {
			t.Errorf("GetRow(0) = %v, want [Alice 30 Boston]", row)
		}
	})

	t.Run("quoted comma in data is not a separator", func(t *testing.T) {
		tb, _ := NewCSV("Name,Comment")
		if err := tb.AddCSVRow(`Alice,"looks good, right?"`); err != nil {
			t.Fatalf("AddCSVRow with quoted comma: %v", err)
		}

		row, _ := tb.GetRow(0)
		if row[1] != "looks good, right?" {
			t.Errorf("expected quoted comma in value, got %q", row[1])
		}
	})

	t.Run("wrong column count returns error", func(t *testing.T) {
		tb, _ := NewCSV("Name,Age")
		// Only one field in the CSV row, but table has two columns.
		if err := tb.AddCSVRow("Alice"); err == nil {
			t.Fatal("expected ErrColumnCount for wrong field count, got nil")
		}
	})

	t.Run("extra field returns error", func(t *testing.T) {
		tb, _ := NewCSV("Name,Age")
		if err := tb.AddCSVRow("Alice,30,Boston"); err == nil {
			t.Fatal("expected ErrColumnCount for extra field, got nil")
		}
	})

	t.Run("multiple rows accumulate correctly", func(t *testing.T) {
		tb, _ := NewCSV("x,y")
		_ = tb.AddCSVRow("1,2")
		_ = tb.AddCSVRow("3,4")

		if tb.Len() != 2 {
			t.Fatalf("expected 2 rows, got %d", tb.Len())
		}
	})
}

// ---------------------------------------------------------------------------
// NewCSV edge cases
// ---------------------------------------------------------------------------

func TestNewCSV_EmptyString(t *testing.T) {
	// An empty input produces a zero-column table (CsvSplit returns nil,
	// which is treated as an empty slice by New).
	tb, err := NewCSV("")
	if err != nil {
		t.Fatalf("NewCSV(\"\") unexpected error: %v", err)
	}

	if tb.Width() != 0 {
		t.Errorf("expected 0 columns, got %d", tb.Width())
	}
}
