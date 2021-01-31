package datatypes

import (
	"testing"
)

func TestNewChannel(t *testing.T) {

	fakeID := "49473e93-9f74-4c88-9234-5e037f2bac13"

	type args struct {
		size int
	}
	tests := []struct {
		name string
		args args
		want *Channel
	}{
		{
			name: "single item channel",
			args: args{size: 1},
			want: &Channel{
				size:   1,
				isOpen: true,
				count:  0,
				id:     fakeID,
			},
		},
		{
			name: "multi item channel",
			args: args{size: 5},
			want: &Channel{
				size:   5,
				isOpen: true,
				count:  0,
				id:     fakeID,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got := NewChannel(tt.args.size)
			// Compare what we can.
			match := true
			if got.count != tt.want.count ||
				got.isOpen != tt.want.isOpen ||
				got.size != tt.want.size {
				match = false
			}
			if !match {
				t.Errorf("NewChannel() = %v, want %v", got, tt.want)
			}
		})
	}
}
