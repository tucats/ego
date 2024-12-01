package tables

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/defs"
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
			ttable := &tt.table

			err := ttable.AddRow(tt.args.row)
			if (err != nil) != tt.wantErr {
				t.Errorf("Table.AddRow() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(*ttable, tt.want) {
				t.Errorf("Table.AddRow() got %v, want %v", ttable, tt.want)
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
