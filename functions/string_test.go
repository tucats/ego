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
		{
			name: "unicode string",
			args: args{[]interface{}{"\u2318foo\u2318", 3}},
			want: "\u2318fo",
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
		{
			name: "unicode string",
			args: args{[]interface{}{"\u2318foo\u2318", 3}},
			want: "oo\u2318",
		},
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
			want: 1,
		},
		{
			name: "array not found",
			args: args{[]interface{}{[]interface{}{"tom", 3.14, true}, false}},
			want: -1,
		},
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

func TestSubstring(t *testing.T) {
	tests := []struct {
		name    string
		args    []interface{}
		want    interface{}
		wantErr bool
	}{
		{
			name: "left case",
			args: []interface{}{"simple", 1, 3},
			want: "sim",
		},
		{
			name: "right case",
			args: []interface{}{"simple", 3, 4},
			want: "mple",
		},
		{
			name: "middle case",
			args: []interface{}{"simple", 3, 1},
			want: "m",
		},
		{
			name: "invalid start case",
			args: []interface{}{"simple", -5, 3},
			want: "sim",
		},
		{
			name: "invalid len case",
			args: []interface{}{"simple", 1, 355},
			want: "simple",
		},
		{
			name: "simple ASCII string starting at 1",
			args: []interface{}{"foobar", 1, 3},
			want: "foo",
		},
		{
			name: "simple ASCII string starting at 3",
			args: []interface{}{"foobar", 3, 2},
			want: "ob",
		},
		{
			name: "simple ASCII string with zero len",
			args: []interface{}{"foobar", 3, 0},
			want: "",
		},
		{
			name: "simple ASCII string with len too big",
			args: []interface{}{"foobar", 3, 10},
			want: "obar",
		},
		{
			name: "Unicode string with zero len",
			args: []interface{}{"\u2318foo\u2318", 3, 0},
			want: "",
		},
		{
			name: "Unicode string starting at 1",
			args: []interface{}{"\u2318foo\u2318", 1, 3},
			want: "\u2318fo",
		},
		{
			name: "Unicode string starting at 2",
			args: []interface{}{"\u2318foo\u2318", 2, 3},
			want: "foo",
		},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Substring(nil, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Substring() error = %v, wantErr %v", err, tt.wantErr)

				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Substring() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseURLPattern(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		pattern string
		want    map[string]interface{}
		matches bool
	}{
		{
			name:    "constant pattern unused segment",
			url:     "/service/debug",
			pattern: "/service/debug/age",
			want: map[string]interface{}{
				"service": true,
				"debug":   true,
				"age":     false,
			},
			matches: true,
		},
		{
			name:    "constant pattern unused sub",
			url:     "/service/debug",
			pattern: "/service/debug/{{age}}",
			want: map[string]interface{}{
				"service": true,
				"debug":   true,
				"age":     "",
			},
			matches: true,
		},
		{
			name:    "constant pattern matches",
			url:     "/service/debug",
			pattern: "/service/debug",
			want: map[string]interface{}{
				"service": true,
				"debug":   true,
			},
			matches: true,
		},
		{
			name:    "constant pattern does not match",
			url:     "/service/debug",
			pattern: "/service/debugz",
			want:    nil,
			matches: false,
		},
		{
			name:    "constant pattern trailing separator mismatch",
			url:     "/service/debug/",
			pattern: "/service/debug",
			want:    nil,
			matches: false,
		},
		{
			name:    "one sub pattern matches",
			url:     "/service/proc/1653",
			pattern: "/service/proc/{{pid}}",
			want: map[string]interface{}{
				"service": true,
				"proc":    true,
				"pid":     "1653",
			},
			matches: true,
		},
		{
			name:    "case sensitive string matches",
			url:     "/service/proc/Accounts",
			pattern: "/service/proc/{{table}}",
			want: map[string]interface{}{
				"service": true,
				"proc":    true,
				"table":   "Accounts",
			},
			matches: true,
		},
		{
			name:    "two subs pattern matches",
			url:     "/service/proc/1653/window/foobar",
			pattern: "/service/proc/{{pid}}/window/{{name}}",
			want: map[string]interface{}{
				"service": true,
				"proc":    true,
				"window":  true,
				"pid":     "1653",
				"name":    "foobar",
			},
			matches: true,
		},
		{
			name:    "two subs pattern does not match",
			url:     "/service/proc/1653/frame/foobar",
			pattern: "/service/proc/{{pid}}/window/{{name}}",
			want:    nil,
			matches: false,
		},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := ParseURLPattern(tt.url, tt.pattern)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseURLPattern() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.matches {
				t.Errorf("ParseURLPattern() got1 = %v, want %v", got1, tt.matches)
			}
		})
	}
}
