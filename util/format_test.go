package util

import "testing"

func TestFormat(t *testing.T) {
	tests := []struct {
		name string
		arg  interface{}
		want string
	}{
		{
			name: "Integer",
			arg:  33,
			want: "33",
		},
		{
			name: "Float",
			arg:  10.5,
			want: "10.5",
		},
		{
			name: "Array of int",
			arg:  []interface{}{3, 5, 55},
			want: "[3 5 55]",
		},
		{
			name: "Array with array",
			arg:  []interface{}{3, []interface{}{"tom", true}, 55},
			want: "[3 [tom true] 55]",
		},
		{
			name: "simple structure",
			arg: map[string]interface{}{
				"name": "Tom",
				"age":  59,
			},
			want: "kind map map[age:59 name:Tom]",
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format(tt.arg); got != tt.want {
				t.Errorf("Format() = %v, want %v", got, tt.want)
			}
		})
	}
}
