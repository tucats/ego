package functions

import (
	"reflect"
	"testing"
)

func TestFunctionLeft(t *testing.T) {
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
			name: "simple test",
			args: args{[]interface{}{"Abraham", 4}},
			want: "Abra",
		},
		{
			name: "negative length test",
			args: args{[]interface{}{"Abraham", -5}},
			want: "",
		},
		{
			name: "length too long test",
			args: args{[]interface{}{"Abraham", 50}},
			want: "Abraham",
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Left(nil, tt.args.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionLeft() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionLeft() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFunctionRight(t *testing.T) {
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
			name: "simple test",
			args: args{[]interface{}{"Abraham", 3}},
			want: "ham",
		},
		{
			name: "length too small test",
			args: args{[]interface{}{"Abraham", -5}},
			want: "",
		},
		{
			name: "length too long test",
			args: args{[]interface{}{"Abraham", 103}},
			want: "Abraham",
		},
		{
			name: "empty string test",
			args: args{[]interface{}{"", 3}},
			want: "",
		},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Right(nil, tt.args.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionRight() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionRight() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFunctionLower(t *testing.T) {
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
			name: "lower case",
			args: args{[]interface{}{"short"}},
			want: "short",
		},
		{
			name: "upper case",
			args: args{[]interface{}{"TALL"}},
			want: "tall",
		},
		{
			name: "mixed case",
			args: args{[]interface{}{"camelCase"}},
			want: "camelcase",
		},
		{
			name: "empty string",
			args: args{[]interface{}{""}},
			want: "",
		},
		{
			name: "non-string",
			args: args{[]interface{}{3.14}},
			want: "3.14",
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Lower(nil, tt.args.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionLower() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionLower() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFunctionUpper(t *testing.T) {
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
			name: "lower case",
			args: args{[]interface{}{"short"}},
			want: "SHORT",
		},
		{
			name: "upper case",
			args: args{[]interface{}{"TALL"}},
			want: "TALL",
		},
		{
			name: "mixed case",
			args: args{[]interface{}{"camelCase"}},
			want: "CAMELCASE",
		},
		{
			name: "empty string",
			args: args{[]interface{}{""}},
			want: "",
		},
		{
			name: "non-string",
			args: args{[]interface{}{3.14}},
			want: "3.14",
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Upper(nil, tt.args.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionUpper() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionUpper() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFunctionSubstring(t *testing.T) {
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
			name: "left case",
			args: args{[]interface{}{"simple", 1, 3}},
			want: "sim",
		},
		{
			name: "right case",
			args: args{[]interface{}{"simple", 3, 4}},
			want: "mple",
		},
		{
			name: "middle case",
			args: args{[]interface{}{"simple", 3, 1}},
			want: "m",
		},
		{
			name: "invalid start case",
			args: args{[]interface{}{"simple", -5, 3}},
			want: "sim",
		},
		{
			name: "invalid len case",
			args: args{[]interface{}{"simple", 1, 355}},
			want: "simple",
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Substring(nil, tt.args.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionSubstring() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionSubstring() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFunctionIndex(t *testing.T) {
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
			name: "index found",
			args: args{[]interface{}{"string of text", "of"}},
			want: 8,
		},
		{
			name: "index not found",
			args: args{[]interface{}{"string of text", "burp"}},
			want: 0,
		},
		{
			name: "empty source string",
			args: args{[]interface{}{"", "burp"}},
			want: 0,
		},
		{
			name: "empty test string",
			args: args{[]interface{}{"string of text", ""}},
			want: 1,
		},
		{
			name: "non-string test",
			args: args{[]interface{}{"A1B2C3D4", 3}},
			want: 6,
		},
		{
			name: "array index",
			args: args{[]interface{}{[]interface{}{"tom", 3.14, true}, 3.14}},
			want: 2,
		},
		{
			name: "array not found",
			args: args{[]interface{}{[]interface{}{"tom", 3.14, true}, false}},
			want: 0,
		},
		{
			name: "member found",
			args: args{[]interface{}{map[string]interface{}{"name": "tom", "age": 55}, "age"}},
			want: true,
		},
		{
			name: "member found",
			args: args{[]interface{}{map[string]interface{}{"name": "tom", "age": 55}, "gender"}},
			want: false,
		},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Index(nil, tt.args.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("FunctionIndex() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FunctionIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}
