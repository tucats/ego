package bytecode

import "testing"

func TestFormatStack(t *testing.T) {
	type args struct {
		s []interface{}
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty stack",
			args: args{s: []interface{}{}},
			want: "<empty>",
		},

		{
			name: "single item",
			args: args{s: []interface{}{55}},
			want: "55",
		},
		{
			name: "two items",
			args: args{s: []interface{}{true, 55}},
			want: "55, true",
		},
		{
			name: "too long",
			args: args{s: []interface{}{1.2345678, "stormtroopers", "the tunafish is in the piano", 55}},
			want: "55, \"the tunafish is in the piano\", \"stormtroopers...",
		},

		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatStack(nil, tt.args.s, false); got != tt.want {
				t.Errorf("FormatStack() = %v, want %v", got, tt.want)
			}
		})
	}
}
