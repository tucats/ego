package commands

import (
	"reflect"
	"testing"
	"time"
)

func Test_parseSequence(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		want    []int
		wantErr bool
	}{
		{
			name: "single number",
			arg:  "22",
			want: []int{22},
		},
		{
			name: "list of numbers",
			arg:  "22, 42",
			want: []int{22, 42},
		},
		{
			name: "range of numbers",
			arg:  "5:7",
			want: []int{5, 6, 7},
		},
		{
			name: "range of numbers with no start",
			arg:  ":3",
			want: []int{1, 2, 3},
		},
		{
			name: "range of numbers with no end",
			arg:  "11:",
			want: []int{11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		},
		{
			name:    "invalid single number",
			arg:     "bob",
			wantErr: true,
		},
		{
			name:    "invalid range start",
			arg:     "*:5",
			wantErr: true,
		},
		{
			name:    "invalid range end",
			arg:     "10:bob",
			wantErr: true,
		},
		{
			name:    "invalid range direction",
			arg:     "10:5",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSequence(tt.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSequence() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseSequence() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test_normalizeLogQueryTime covers the --since/--until normalization that
// feeds the /admin/log endpoint's "since" and "until" query parameters: a
// flexibly-parsed date/time value should always come out as valid RFC 3339,
// and an unparseable value should be rejected rather than silently ignored.
func Test_normalizeLogQueryTime(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		wantErr bool
	}{
		{
			name: "date only",
			arg:  "2026-08-12",
		},
		{
			name: "date and time",
			arg:  "2026-08-12 10:15:00",
		},
		{
			name: "RFC 3339",
			arg:  "2026-08-12T10:15:00Z",
		},
		{
			name: "RFC 3339 with offset",
			arg:  "2026-08-12T10:15:00-07:00",
		},
		{
			name:    "not a date",
			arg:     "not-a-date",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeLogQueryTime(tt.arg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeLogQueryTime() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if _, err := time.Parse(time.RFC3339, got); err != nil {
				t.Errorf("normalizeLogQueryTime(%q) = %q, not valid RFC3339: %v", tt.arg, got, err)
			}
		})
	}
}
