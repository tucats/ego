package util

import (
	"testing"

	"github.com/google/uuid"
)

func TestGibberish(t *testing.T) {
	// Set up test cases
	tests := []struct {
		name string
		u    uuid.UUID
		want string
	}{
		{
			name: "test 1 synthetic UUID",
			u:    uuid.MustParse("00000000-0000-0000-0000-000000000000"),
			want: "-empty-",
		},
		{
			name: "test 2 synthetic UUID",
			u:    uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			want: "b",
		},
		{
			name: "test 3 synthetic UUID",
			u:    uuid.MustParse("10000000-0000-0000-0000-000000000000"),
			want: "aaaaaaaaaaaab",
		},
		{
			name: "test 1 random UUID",
			u:    uuid.MustParse("ab34d542-a437-408a-b0ca-38ea5d78696f"),
			want: "rm4szqj72tubkesqdukixgpyk",
		},
		{
			name: "test 2 random UUID",
			u:    uuid.MustParse("4867dd02-3b98-4d68-9843-06179aa8553e"),
			want: "8jxskp8cg2simvs37ia783se",
		},
	}

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Gibberish(tt.u)
			if got != tt.want {
				t.Errorf("Gibberish(%s) = %v, want %v", tt.u, got, tt.want)
			}
		})
	}
}
