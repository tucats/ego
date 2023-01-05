package tables

import (
	"reflect"
	"testing"
)

func TestNew(t *testing.T) {
	type args struct {
		headings []string
	}

	tests := []struct {
		name      string
		args      args
		want      *Table
		wantError bool
	}{
		{
			name: "Simple table with one column",
			args: args{
				headings: []string{"simple"},
			},
			want: &Table{
				rowLimit:       -1,
				columnCount:    1,
				names:          []string{"simple"},
				maxWidth:       []int{6},
				alignment:      []int{AlignmentLeft},
				spacing:        "    ",
				indent:         "",
				rows:           make([][]string, 0),
				columnOrder:    []int{0},
				orderBy:        -1,
				ascending:      true,
				showUnderlines: true,
				showHeadings:   true,
			},
			wantError: false,
		},
		{
			name: "Simple table with three columns",
			args: args{
				headings: []string{"simple", "test", "table"},
			},
			want: &Table{
				rowLimit:       -1,
				columnCount:    3,
				names:          []string{"simple", "test", "table"},
				maxWidth:       []int{6, 4, 5},
				alignment:      []int{AlignmentLeft, AlignmentLeft, AlignmentLeft},
				columnOrder:    []int{0, 1, 2},
				spacing:        "    ",
				indent:         "",
				rows:           make([][]string, 0),
				orderBy:        -1,
				ascending:      true,
				showUnderlines: true,
				showHeadings:   true,
			},
			wantError: false,
		},
		{
			name: "Invalid table with no columns",
			args: args{
				headings: []string{},
			},
			want:      &Table{},
			wantError: true,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.args.headings)
			if err != nil && !tt.wantError {
				t.Errorf("New() resulted in unexpected error %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewCSV(t *testing.T) {
	type args struct {
		h string
	}

	tests := []struct {
		name    string
		args    args
		want    Table
		wantErr bool
	}{
		{
			name: "Simple case with one column",
			args: args{
				h: "First",
			},
			want: Table{
				names: []string{"First"},
			},
			wantErr: false,
		},
		{
			name: "Simple case with two columns",
			args: args{
				h: "First,Second",
			},
			want: Table{
				names: []string{"First", "Second"},
			},
			wantErr: false,
		},
		{
			name: "Empty column name",
			args: args{
				h: "First,,Third",
			},
			want: Table{
				names: []string{"First", "", "Third"},
			},
			wantErr: false,
		},
		{
			name: "Extra spaces",
			args: args{
				h: "First, Second  ",
			},
			want: Table{
				names: []string{"First", "Second"},
			},
			wantErr: false,
		},
		{
			name: "Quoted commas",
			args: args{
				h: "\"Name,Age\",Size",
			},
			want: Table{
				names: []string{"Name,Age", "Size"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewCSV(tt.args.h)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCSV() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got.names, tt.want.names) {
				t.Errorf("NewCSV() = %v, want %v", got, tt.want)
			}
		})
	}
}
