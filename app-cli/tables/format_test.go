package tables

import (
	"reflect"
	"testing"
)

func TestTable_SetAlignment(t *testing.T) {
	type fields struct {
		showUnderlines bool
		showHeadings   bool
		showRowNumbers bool
		rowLimit       int
		startingRow    int
		columnCount    int
		rowCount       int
		orderBy        int
		ascending      bool
		rows           [][]string
		columns        []string
		active         []bool
		alignment      []int
		maxWidth       []int
		spacing        string
		indent         string
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
			name: "Set formatting for invalive negatie column",
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
		// TODO: Add test cases.
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
				rowCount:       tt.fields.rowCount,
				orderBy:        tt.fields.orderBy,
				ascending:      tt.fields.ascending,
				rows:           tt.fields.rows,
				columns:        tt.fields.columns,
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
		showUnderlines bool
		showHeadings   bool
		showRowNumbers bool
		rowLimit       int
		startingRow    int
		columnCount    int
		rowCount       int
		orderBy        int
		ascending      bool
		rows           [][]string
		columns        []string
		alignment      []int
		maxWidth       []int
		spacing        string
		indent         string
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
			name: "Inalid spacing",
			args: args{
				s: -5,
			},
			wantErr:     true,
			wantSpacing: "",
		},
		// TODO: Add test cases.
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
				rowCount:       tt.fields.rowCount,
				orderBy:        tt.fields.orderBy,
				ascending:      tt.fields.ascending,
				rows:           tt.fields.rows,
				columns:        tt.fields.columns,
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
		showUnderlines bool
		showHeadings   bool
		showRowNumbers bool
		rowLimit       int
		startingRow    int
		columnCount    int
		rowCount       int
		orderBy        int
		ascending      bool
		rows           [][]string
		columns        []string
		alignment      []int
		maxWidth       []int
		spacing        string
		indent         string
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
			name: "Inalid indent",
			args: args{
				s: -5,
			},
			wantErr:     true,
			wantSpacing: "",
		},
		// TODO: Add test cases.
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
				rowCount:       tt.fields.rowCount,
				orderBy:        tt.fields.orderBy,
				ascending:      tt.fields.ascending,
				rows:           tt.fields.rows,
				columns:        tt.fields.columns,
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
