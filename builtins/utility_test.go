package builtins

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
)

func TestFunctionLen(t *testing.T) {
	tests := []struct {
		name    string
		args    data.List
		want    interface{}
		wantErr bool
	}{
		{
			name: "string length",
			args: data.NewList("hamster"),
			want: 7,
		},
		{
			name: "empty string length",
			args: data.NewList(""),
			want: 0,
		},
		{
			name:    "numeric value length",
			args:    data.NewList(3.14),
			want:    0,
			wantErr: true,
		},
		{
			name: "array length",
			args: data.NewList(
				data.NewArrayFromList(
					data.InterfaceType,
					data.NewList(
						true,
						3.14,
						"Tom",
					),
				),
			),
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Length(nil, tt.args)
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
		args    data.List
		want    interface{}
		wantErr bool
	}{
		{
			name: "simple string",
			args: data.NewList("foo"),
			want: 3,
		},
		{
			name: "unicode string",
			args: data.NewList("\u2318foo\u2318"),
			want: 9,
		},
		{
			name: "simple array",
			args: data.NewList(
				data.NewArrayFromList(data.IntType, data.NewList(1, 2, 3, 4)),
			),
			want: 4,
		},
		{
			name: "simple map",
			args: data.NewList(
				data.NewMapFromMap(
					map[string]interface{}{
						"name": "Bob",
						"age":  35,
					}),
			),
			want: 2,
		},
		{
			name: "int converted to string",
			args: data.NewList("123456"),
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
