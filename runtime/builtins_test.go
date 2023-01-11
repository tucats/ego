package runtime

import (
	"testing"
)

func Test_pad(t *testing.T) {
	type args struct {
		s string
		w int
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "pad left",
			args: args{s: "tom", w: 5},
			want: "tom  ",
		},
		{
			name: "pad right",
			args: args{s: "tom", w: -5},
			want: "  tom",
		},
		{
			name: "empty",
			args: args{s: "", w: 5},
			want: "     ",
		},
		{
			name: "truncate",
			args: args{s: "ezekiel", w: 5},
			want: "ezeki",
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Pad(tt.args.s, tt.args.w); got != tt.want {
				t.Errorf("pad() = %v, want %v", got, tt.want)
			}
		})
	}
}
