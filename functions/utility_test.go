package functions

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
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
			name: "numeric value length",
			args: args{[]interface{}{3.14}},
			want: 4,
		},
		{
			name: "array length",
			args: args{
				[]interface{}{
					datatypes.NewArrayFromArray(
						&datatypes.InterfaceType,
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
			if (!errors.Nil(err)) != tt.wantErr {
				t.Errorf("FunctionLen() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionLen() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFunctionSort(t *testing.T) {
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
			name: "scalar args",
			args: args{
				[]interface{}{66, 55},
			},
			want:    []interface{}{55, 66},
			wantErr: false,
		},
		{
			name:    "mixed scalar args",
			args:    args{[]interface{}{"tom", 3}},
			want:    []interface{}{"3", "tom"},
			wantErr: false,
		},
		{
			name: "integer sort",
			args: args{[]interface{}{
				datatypes.NewArrayFromArray(&datatypes.IntType, []interface{}{55, 2, 18})},
			},
			want: []interface{}{2, 18, 55},
		},
		{
			name: "float64 sort",
			args: args{[]interface{}{
				datatypes.NewArrayFromArray(&datatypes.Float64Type, []interface{}{55.0, 2, 18.5})}},
			want: []interface{}{2.0, 18.5, 55.0},
		},
		{
			name: "string sort",
			args: args{[]interface{}{
				datatypes.NewArrayFromArray(&datatypes.StringType, []interface{}{"pony", "cake", "unicorn", 5})}},
			want: []interface{}{"5", "cake", "pony", "unicorn"},
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Sort(nil, tt.args.args)
			if (!errors.Nil(err)) != tt.wantErr {
				t.Errorf("FunctionSort() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			gotArray, ok := got.(*datatypes.EgoArray)
			if !ok || !reflect.DeepEqual(gotArray.BaseArray(), tt.want) {
				t.Errorf("FunctionSort() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFunctionMembers(t *testing.T) {
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
			name: "simple struct",
			args: args{[]interface{}{
				datatypes.NewStructFromMap(
					map[string]interface{}{"name": "Tom", "age": 55},
				),
			}},
			want: datatypes.NewArrayFromArray(&datatypes.StringType, []interface{}{"age", "name"}),
		},
		{
			name: "empty struct",
			args: args{[]interface{}{datatypes.NewStruct(&datatypes.StructType)}},
			want: datatypes.NewArrayFromArray(&datatypes.StringType, []interface{}{}),
		},
		{
			name:    "wrong type struct",
			args:    args{[]interface{}{55}},
			want:    nil,
			wantErr: true,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Members(nil, tt.args.args)
			if (!errors.Nil(err)) != tt.wantErr {
				t.Errorf("FunctionMembers() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionMembers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReflect(t *testing.T) {
	type args struct {
		s    *symbols.SymbolTable
		args []interface{}
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "simple struct",
			args: args{
				s: nil,
				args: []interface{}{
					datatypes.NewStructFromMap(map[string]interface{}{
						"name": "Tom",
						"age":  55,
					}),
				},
			},
			want: datatypes.NewStructFromMap(map[string]interface{}{
				"basetype": "struct{age int, name string}",
				"type":     "struct{age int, name string}",
				"native":   true,
				"readonly": false,
				"static":   true,
				"istype":   false,
				"members":  datatypes.NewArrayFromArray(&datatypes.StringType, []interface{}{"age", "name"}),
			}),
			wantErr: false,
		},
		{
			name: "simple integer value",
			args: args{s: nil, args: []interface{}{33}},
			want: datatypes.NewStructFromMap(map[string]interface{}{
				"basetype": "int",
				"type":     "int",
				"istype":   false,
			}),
			wantErr: false,
		},
		{
			name: "simple string value",
			args: args{s: nil, args: []interface{}{"stuff"}},
			want: datatypes.NewStructFromMap(map[string]interface{}{
				"basetype": "string",
				"type":     "string",
				"istype":   false,
			}),
			wantErr: false,
		},
		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Reflect(tt.args.s, tt.args.args)
			if (!errors.Nil(err)) != tt.wantErr {
				t.Errorf("Reflect() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Reflect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStrLen(t *testing.T) {
	tests := []struct {
		name    string
		args    []interface{}
		want    interface{}
		wantErr bool
	}{
		{
			name: "length of ASCII string",
			args: []interface{}{"foo"},
			want: 3,
		},
		{
			name: "length of empty string",
			args: []interface{}{""},
			want: 0,
		},
		{
			name: "length of Unicode string",
			args: []interface{}{"\u2318Foo\u2318"},
			want: 5,
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StrLen(nil, tt.args)
			if (!errors.Nil(err)) != tt.wantErr {
				t.Errorf("StrLen() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StrLen() = %v, want %v", got, tt.want)
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
				datatypes.NewArrayFromArray(&datatypes.IntType, []interface{}{1, 2, 3, 4}),
			},
			want: 4,
		},
		{
			name: "simple map",
			args: []interface{}{
				datatypes.NewMapFromMap(
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
			if (!errors.Nil(err)) != tt.wantErr {
				t.Errorf("Length() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Length() = %v, want %v", got, tt.want)
			}
		})
	}
}
