package tables

import (
	"testing"
)

// getExcludeList builds the set of columns to leave OUT of a result by marking
// every column excluded and then un-excluding the ones the caller named. That
// construction makes an empty column list dangerous: with nothing to
// un-exclude, every column stays excluded and the caller gets rows with no
// columns in them.
//
// The "list" parameter type used to reject a present-but-empty value
// (?columns=) before it reached the handler. It now accepts one, because an
// empty list means the caller is not filtering -- so the handler has to
// recognise that itself. hasAnyName is the guard that does it, and this pins
// its behavior without needing a live database to build the column list from.
func TestHasAnyName(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want bool
	}{
		{
			name: "parameter absent",
			in:   nil,
			want: false,
		},
		{
			name: "parameter present but empty is not a column filter",
			in:   []string{""},
			want: false,
		},
		{
			name: "only separators is not a column filter",
			in:   []string{",,"},
			want: false,
		},
		{
			name: "only spaces is not a column filter",
			in:   []string{"  "},
			want: false,
		},
		{
			name: "a single column name",
			in:   []string{"name"},
			want: true,
		},
		{
			name: "comma-separated column names",
			in:   []string{"name,age"},
			want: true,
		},
		{
			name: "the parameter repeated",
			in:   []string{"name", "age"},
			want: true,
		},
		{
			name: "quoted column names still count",
			in:   []string{"\"name\""},
			want: true,
		},
		{
			name: "an empty value alongside a real one still counts",
			in:   []string{"", "name"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasAnyName(tt.in); got != tt.want {
				t.Errorf("hasAnyName(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
