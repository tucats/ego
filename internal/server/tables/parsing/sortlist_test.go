package parsing

import (
	"net/url"
	"strings"
	"testing"
)

// SortList turns a ?sort= query parameter into an SQL ORDER BY clause.
//
// The case that matters here is a sort parameter that is present but empty.
// The "list" parameter type used to reject that outright, so it never reached
// this function; it now accepts it, because an empty list means the caller is
// not filtering. SortList has to agree: an empty sort is no sort at all. Emitting
// a bare "ORDER BY" with no column after it would be invalid SQL.
func TestSortListTreatsEmptyAsNoSort(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "no sort parameter at all",
			url:  "http://localhost:8080/tables/x/rows",
			want: "",
		},
		{
			name: "sort parameter present but empty",
			url:  "http://localhost:8080/tables/x/rows?sort=",
			want: "",
		},
		{
			name: "sort parameter repeated but all empty",
			url:  "http://localhost:8080/tables/x/rows?sort=&sort=",
			want: "",
		},
		{
			name: "a real sort still works",
			url:  "http://localhost:8080/tables/x/rows?sort=name",
			want: "ORDER BY name",
		},
		{
			name: "a descending sort still works",
			url:  "http://localhost:8080/tables/x/rows?sort=~name",
			want: "ORDER BY name DESC",
		},
		{
			name: "an empty value alongside a real one is ignored",
			url:  "http://localhost:8080/tables/x/rows?sort=&sort=name",
			want: "ORDER BY name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("could not parse test URL: %v", err)
			}

			got := SortList(parsed)

			if got != tt.want {
				t.Errorf("SortList(%s) = %q, want %q", tt.url, got, tt.want)
			}

			// Whatever else happens, never emit an ORDER BY with nothing after it.
			if strings.TrimSpace(got) == "ORDER BY" {
				t.Errorf("SortList(%s) produced a bare ORDER BY, which is not valid SQL", tt.url)
			}
		})
	}
}
