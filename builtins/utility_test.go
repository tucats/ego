package builtins

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
)

func TestFunctionLen(t *testing.T) {
	type args struct {
		args []interface{}
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "string length",
			args: args{[]interface{}{"hamster"}},
			want: 7,
		},
		{
			name: "empty string length",
			args: args{[]interface{}{""}},
			want: 0,
		},
		{
			name:    "numeric value length",
			args:    args{[]interface{}{3.14}},
			want:    0,
			wantErr: true,
		},
		{
			name: "array length",
			args: args{
				[]interface{}{
					data.NewArrayFromArray(
						data.InterfaceType,
						[]interface{}{
							true,
							3.14,
							"Tom",
						}),
				},
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Length(nil, tt.args.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionLen() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionLen() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLength(t *testing.T) {
	tests := []struct {
		name    string
		args    []interface{}
		want    interface{}
		wantErr bool
	}{
		{
			name: "simple string",
			args: []interface{}{"foo"},
			want: 3,
		},
		{
			name: "unicode string",
			args: []interface{}{"\u2318foo\u2318"},
			want: 9,
		},
		{
			name: "simple array",
			args: []interface{}{
				data.NewArrayFromArray(data.IntType, []interface{}{1, 2, 3, 4}),
			},
			want: 4,
		},
		{
			name: "simple map",
			args: []interface{}{
				data.NewMapFromMap(
					map[string]interface{}{
						"name": "Bob",
						"age":  35,
					}),
			},
			want: 2,
		},
		{
			name: "int converted to string",
			args: []interface{}{"123456"},
			want: 6,
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Length(nil, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Length() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Length() = %v, want %v", got, tt.want)
			}
		})
	}
}
