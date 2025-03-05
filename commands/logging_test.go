package commands

import (
	"reflect"
	"testing"
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
