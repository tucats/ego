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
			args: args{[]interface{}{[]interface{}{true, 3.14, "Tom"}}},
			want: 3,
		},
		{
			name: "struct value length",
			args: args{[]interface{}{map[string]interface{}{"name": "Tom", "age": 33}}},
			want: 2,
		},
		// TODO: Add test cases.
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

func TestFunctionProfile(t *testing.T) {
	type args struct {
		args []interface{}
	}

	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{

		// Tests create an arbitrary key using a static UUID
		{
			name: "crete a key",
			args: args{[]interface{}{"b306e250-6e07-4a05-abf4-e6a64d64cb72", "cookies"}},
			want: nil,
		},
		{
			name: "read a key",
			args: args{[]interface{}{"b306e250-6e07-4a05-abf4-e6a64d64cb72"}},
			want: "cookies",
		},
		{
			name: "delete a key",
			args: args{[]interface{}{"b306e250-6e07-4a05-abf4-e6a64d64cb72", ""}},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got interface{}
			var err error
			if len(tt.args.args) > 1 {
				got, err = ProfileSet(nil, tt.args.args)
			} else {
				got, err = ProfileGet(nil, tt.args.args)
			}
			if (!errors.Nil(err)) != tt.wantErr {
				t.Errorf("FunctionProfile() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionProfile() = %v, want %v", got, tt.want)
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
			name:    "scalar args",
			args:    args{[]interface{}{66, 55}},
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
			args: args{[]interface{}{[]interface{}{55, 2, 18}}},
			want: []interface{}{2, 18, 55},
		},
		{
			name: "float sort",
			args: args{[]interface{}{[]interface{}{55.0, 2, "18.5"}}},
			want: []interface{}{2.0, 18.5, 55.0},
		},
		{
			name: "string sort",
			args: args{[]interface{}{[]interface{}{"pony", "cake", "unicorn", 5}}},
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
			args: args{[]interface{}{map[string]interface{}{"name": "Tom", "age": 55}}},
			want: datatypes.NewFromArray(datatypes.StringType, []interface{}{"age", "name"}),
		},
		{
			name: "empty struct",
			args: args{[]interface{}{map[string]interface{}{}}},
			want: datatypes.NewFromArray(datatypes.StringType, []interface{}{}),
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
			args: args{s: nil, args: []interface{}{
				map[string]interface{}{
					"name": "Tom",
					"age":  55,
				},
			}},
			want: map[string]interface{}{
				"basetype": "map",
				"type":     "struct",
				"members":  datatypes.NewFromArray(datatypes.StringType, []interface{}{"age", "name"}),
			},
			wantErr: false,
		},
		{
			name: "simple integer value",
			args: args{s: nil, args: []interface{}{33}},
			want: map[string]interface{}{
				"basetype": "int",
				"type":     "int",
			},
			wantErr: false,
		},
		{
			name: "simple string value",
			args: args{s: nil, args: []interface{}{"stuff"}},
			want: map[string]interface{}{
				"basetype": "string",
				"type":     "string",
			},
			wantErr: false,
		},
		{
			name: "array of ints",
			args: args{s: nil, args: []interface{}{
				[]interface{}{1, 2, 3},
			}},
			want: map[string]interface{}{
				"basetype": "array",
				"type":     "[]int",
				"elements": "int",
				"size":     3,
			},
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
				[]interface{}{1, 2, 3, 4},
			},
			want: 4,
		},
		{
			name: "simple map",
			args: []interface{}{
				map[string]interface{}{
					"name": "Bob",
					"age":  35,
				},
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
