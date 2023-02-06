package data

import "testing"

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "foobar",
			want: "foobar",
		},
		{
			name: "foo\nbar",
			want: "foo.bar",
		},
		{
			name: "fo\to\nbar",
			want: "fo.o.bar",
		},
		{
			name: "$foobar",
			want: ".foobar",
		},
		{
			name: "foo\\bar",
			want: "foo.bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeName(tt.name); got != tt.want {
				t.Errorf("SanitizeName() = %v, want %v", got, tt.want)
			}
		})
	}
}
