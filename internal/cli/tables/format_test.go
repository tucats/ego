package tables

import (
	"reflect"
	"strings"
	"testing"
)

func TestTable_SetAlignment(t *testing.T) {
	type fields struct {
		rows           [][]string
		columns        []string
		alignment      []int
		maxWidth       []int
		active         []bool
		spacing        string
		indent         string
		rowLimit       int
		startingRow    int
		columnCount    int
		orderBy        int
		showUnderlines bool
		showHeadings   bool
		showRowNumbers bool
		ascending      bool
	}

	type args struct {
		column    int
		alignment int
	}

	tests := []struct {
		name          string
		fields        fields
		args          args
		wantErr       bool
		wantAlignment []int
	}{
		{
			name: "Set formatting for existing column",
			fields: fields{
				columnCount: 1,
				alignment:   make([]int, 1),
				active:      []bool{true},
			},
			args: args{
				column:    0,
				alignment: AlignmentLeft,
			},
			wantAlignment: []int{AlignmentLeft},
		},
		{
			name: "Set formatting for non-existent column",
			fields: fields{
				columnCount: 1,
				alignment:   make([]int, 1),
				active:      []bool{true},
			},
			args: args{
				column:    15,
				alignment: AlignmentLeft,
			},
			wantAlignment: []int{AlignmentLeft},
			wantErr:       true,
		},
		{
			name: "Set formatting for invalid negative column",
			fields: fields{
				columnCount: 1,
				alignment:   make([]int, 1),
				active:      []bool{true},
			},
			args: args{
				column:    -3,
				alignment: AlignmentLeft,
			},
			wantAlignment: []int{AlignmentLeft},
			wantErr:       true,
		},
		{
			name: "Set invalid formatting for column",
			fields: fields{
				columnCount: 1,
				alignment:   make([]int, 1),
				active:      []bool{true},
			},
			args: args{
				column:    1,
				alignment: 33,
			},
			wantAlignment: []int{AlignmentLeft},
			wantErr:       true,
		},
		{
			name: "Set formatting for existing column",
			fields: fields{
				columnCount: 1,
				active:      []bool{true},
				alignment:   make([]int, 1),
			},
			args: args{
				column:    0,
				alignment: AlignmentLeft,
			},
			wantAlignment: []int{AlignmentLeft},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := &Table{
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
				alignment:      tt.fields.alignment,
				maxWidth:       tt.fields.maxWidth,
				spacing:        tt.fields.spacing,
				indent:         tt.fields.indent,
			}
			err := tb.SetAlignment(tt.args.column, tt.args.alignment)

			if (err != nil) != tt.wantErr {
				t.Errorf("Table.SetAlignment() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && !reflect.DeepEqual(tb.alignment, tt.wantAlignment) {
				t.Errorf("Table.SetAlignment() alignment = %v, want %v", tb.alignment, tt.wantAlignment)
			}
		})
	}
}

func TestTable_SetSpacing(t *testing.T) {
	type fields struct {
		rows           [][]string
		columns        []string
		alignment      []int
		maxWidth       []int
		spacing        string
		indent         string
		rowLimit       int
		startingRow    int
		columnCount    int
		orderBy        int
		showUnderlines bool
		showHeadings   bool
		showRowNumbers bool
		ascending      bool
	}

	type args struct {
		s int
	}

	tests := []struct {
		name        string
		fields      fields
		args        args
		wantErr     bool
		wantSpacing string
	}{
		{
			name: "Valid spacing",
			args: args{
				s: 5,
			},
			wantErr:     false,
			wantSpacing: "     ",
		},
		{
			name: "Invalid spacing",
			args: args{
				s: -5,
			},
			wantErr:     true,
			wantSpacing: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := &Table{
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
				alignment:      tt.fields.alignment,
				maxWidth:       tt.fields.maxWidth,
				spacing:        tt.fields.spacing,
				indent:         tt.fields.indent,
			}

			err := tb.SetSpacing(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("Table.SetSpacing() error = %v, wantErr %v", err, tt.wantErr)
			} else if tb.spacing != tt.wantSpacing {
				t.Errorf("Table.SetSpacing() error, incorrect number of spaces")
			}
		})
	}
}

func TestTable_SetIndent(t *testing.T) {
	type fields struct {
		rows           [][]string
		columns        []string
		alignment      []int
		maxWidth       []int
		spacing        string
		indent         string
		rowLimit       int
		startingRow    int
		columnCount    int
		orderBy        int
		ascending      bool
		showUnderlines bool
		showHeadings   bool
		showRowNumbers bool
	}

	type args struct {
		s int
	}

	tests := []struct {
		name        string
		fields      fields
		args        args
		wantErr     bool
		wantSpacing string
	}{
		{
			name: "Valid indent",
			args: args{
				s: 5,
			},
			wantErr:     false,
			wantSpacing: "     ",
		},
		{
			name: "Invalid indent",
			args: args{
				s: -5,
			},
			wantErr:     true,
			wantSpacing: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := &Table{
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
				alignment:      tt.fields.alignment,
				maxWidth:       tt.fields.maxWidth,
				spacing:        tt.fields.spacing,
				indent:         tt.fields.indent,
			}
			
			err := tb.SetIndent(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("Table.SetSpacing() error = %v, wantErr %v", err, tt.wantErr)
			} else if tb.indent != tt.wantSpacing {
				t.Errorf("Table.SetSpacing() error, incorrect number of spaces")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SetAlignment — Right and Center (the existing test only covers Left)
// ---------------------------------------------------------------------------

func TestTable_SetAlignment_Right(t *testing.T) {
	tb, _ := New([]string{"n"})
	if err := tb.SetAlignment(0, AlignmentRight); err != nil {
		t.Fatalf("SetAlignment Right: %v", err)
	}

	if tb.alignment[0] != AlignmentRight {
		t.Errorf("alignment[0] = %d, want AlignmentRight (%d)", tb.alignment[0], AlignmentRight)
	}
}

func TestTable_SetAlignment_Center(t *testing.T) {
	tb, _ := New([]string{"n"})
	if err := tb.SetAlignment(0, AlignmentCenter); err != nil {
		t.Fatalf("SetAlignment Center: %v", err)
	}

	if tb.alignment[0] != AlignmentCenter {
		t.Errorf("alignment[0] = %d, want AlignmentCenter (%d)", tb.alignment[0], AlignmentCenter)
	}
}

// The existing TestTable_SetAlignment test case "Set invalid formatting for column"
// uses column=1 on a 1-column table, so it hits the column-range check before
// the alignment check. This test verifies the alignment-value check directly.
func TestTable_SetAlignment_InvalidValue_ValidColumn(t *testing.T) {
	tb, _ := New([]string{"a"})

	if err := tb.SetAlignment(0, 99); err == nil {
		t.Error("expected ErrAlignment for value 99 on valid column 0, got nil")
	}
}

// ---------------------------------------------------------------------------
// SetSpacing — zero is valid (produces no inter-column gap)
// ---------------------------------------------------------------------------

func TestTable_SetSpacing_Zero(t *testing.T) {
	tb, _ := New([]string{"a"})
	if err := tb.SetSpacing(0); err != nil {
		t.Fatalf("SetSpacing(0) unexpected error: %v", err)
	}

	if tb.spacing != "" {
		t.Errorf("spacing = %q after SetSpacing(0), want empty string", tb.spacing)
	}
}

// ---------------------------------------------------------------------------
// SetSpacing / SetIndent — output effect in FormatText
// ---------------------------------------------------------------------------

func TestTable_SetSpacing_OutputEffect(t *testing.T) {
	tb, _ := New([]string{"a", "b"})
	tb.SetPagination(0, 0)
	tb.ShowHeadings(false)
	_ = tb.AddRow([]string{"X", "Y"})

	// With 1-space inter-column spacing the output line should contain
	// exactly one space between the column values.
	if err := tb.SetSpacing(1); err != nil {
		t.Fatalf("SetSpacing(1): %v", err)
	}

	lines := tb.FormatText()
	if len(lines) == 0 {
		t.Fatal("FormatText returned no lines")
	}

	// Line is "X Y" with a single space.
	line := strings.TrimRight(lines[0], " ")
	if line != "X Y" {
		t.Errorf("SetSpacing(1): expected %q, got %q", "X Y", line)
	}
}

func TestTable_SetIndent_OutputEffect(t *testing.T) {
	tb, _ := New([]string{"v"})
	tb.SetPagination(0, 0)
	tb.ShowHeadings(false)
	_ = tb.AddRow([]string{"Z"})

	if err := tb.SetIndent(3); err != nil {
		t.Fatalf("SetIndent(3): %v", err)
	}

	lines := tb.FormatText()
	if len(lines) == 0 {
		t.Fatal("FormatText returned no lines")
	}

	if !strings.HasPrefix(lines[0], "   ") {
		t.Errorf("SetIndent(3): line should start with 3 spaces, got %q", lines[0])
	}
}
