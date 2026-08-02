package caches

import (
	"strings"
	"testing"
)

// namedClasses decides whether DELETE /admin/caches was given a class filter.
//
// Two things are pinned here. First, a "class" parameter that is present but
// empty (?class=) must read the same as one that was never supplied: no filter,
// which for this endpoint means purge everything. The "list" parameter type
// used to reject an empty value before it reached the handler; it now accepts
// one, so the handler has to make that call itself. Purging nothing would be
// the wrong answer -- it would silently do nothing at all.
//
// Second, a list parameter is comma-separated by definition, and this handler
// never split on commas. ?class=user,dsn matched none of the switch cases and
// so purged nothing, which is what the caller least expected.
func TestNamedClasses(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "parameter absent",
			in:   nil,
			want: []string{},
		},
		{
			name: "parameter present but empty means no filter",
			in:   []string{""},
			want: []string{},
		},
		{
			name: "only separators and spaces means no filter",
			in:   []string{" , , "},
			want: []string{},
		},
		{
			name: "a single class",
			in:   []string{"tokens"},
			want: []string{"tokens"},
		},
		{
			name: "comma-separated classes in one value are split",
			in:   []string{"user,dsn"},
			want: []string{"user", "dsn"},
		},
		{
			name: "the parameter repeated",
			in:   []string{"user", "dsn"},
			want: []string{"user", "dsn"},
		},
		{
			name: "spaces around names are trimmed",
			in:   []string{" user , dsn "},
			want: []string{"user", "dsn"},
		},
		{
			name: "an empty value alongside a real one is ignored",
			in:   []string{"", "tokens"},
			want: []string{"tokens"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := namedClasses(tt.in)

			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("namedClasses(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
