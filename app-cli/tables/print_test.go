package tables

import (
	"testing"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
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
		// TODO: Add test cases.
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
				rowCount:       tt.fields.rowCount,
				orderBy:        tt.fields.orderBy,
				ascending:      tt.fields.ascending,
				rows:           tt.fields.rows,
				columns:        tt.fields.columns,
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
		// TODO: Add test cases.
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

		tb, _ := New([]string{
			"First",
			"Last",
			"Address",
			"Description",
			"Relation",
		})

		tb.SetPagination(20, 50)

		tb.AddRowItems(
			"Tom",
			"Stephanofphalosfis",
			"100 North Wakualewaka Lake Drive, Primrose NC 28391",
			"Software inventor",
			"Self",
		)

		tb.AddRowItems(
			"Donna",
			"Wilson",
			"100 Main St, Primrose NC 28391",
			"iPhone Developer",
			"Sister",
		)

		_ = tb.Print(ui.TextFormat)
		//if !errors.Nil(e) {
		//  t.Error(e)
		//}

	})
}
