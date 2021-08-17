package bytecode

import (
	"testing"

	"github.com/tucats/ego/datatypes"
)

func Test_makeArrayByteCode(t *testing.T) {
	type args struct {
		stack []interface{}
		i     int
	}
	tests := []struct {
		name string
		args args
		want *datatypes.EgoArray
	}{
		{
			name: "[]int{5,3}",
			args: args{
				stack: []interface{}{3, 5, datatypes.IntType},
				i:     2,
			},
			want: datatypes.NewArrayFromArray(datatypes.IntType, []interface{}{3, 5}),
		},
		{
			name: "[]string{\"Tom\", \"Cole\"}",
			args: args{
				stack: []interface{}{"Cole", "Tom", datatypes.StringType},
				i:     2,
			},
			want: datatypes.NewArrayFromArray(datatypes.IntType, []interface{}{3, 5}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &Context{stack: tt.args.stack, stackPointer: len(tt.args.stack)}

			e := makeArrayByteCode(ctx, tt.args.i)
			if e != nil {
				t.Errorf("Unexpected error %v", e)
			}

		})
	}
}
