package i18n

import (
	"reflect"
	"testing"
)

func Test_splitString(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want []string
	}{
		{
			name: "sub at end",
			arg:  "string {{item}}",
			want: []string{"string ", "{{item}}"},
		},
		{
			name: "sub at start",
			arg:  "{{item}} string",
			want: []string{"{{item}}", " string"},
		},
		{
			name: "single part",
			arg:  "simple string",
			want: []string{"simple string"},
		},
		{
			name: "one sub",
			arg:  "test {{item}} string",
			want: []string{"test ", "{{item}}", " string"},
		},
		{
			name: "multiple subs",
			arg:  "test {{item1}} and {{item2}} string",
			want: []string{"test ", "{{item1}}", " and ", "{{item2}}", " string"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitOutFormats(tt.arg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_handleSub(t *testing.T) {
	tests := []struct {
		name string
		text string
		subs map[string]interface{}
		want string
	}{
		{
			name: "label zero value",
			text: `{{item|label "flag="}}`,
			subs: map[string]interface{}{"item": 0},
			want: "",
		},
		{
			name: "explicit format string sub",
			text: "{{item|format %10s}}",
			subs: map[string]interface{}{"item": "test"},
			want: "      test",
		},
		{
			name: "label zero value with empty value",
			text: `{{item|label "flag="|%05x | empty none}}`,
			subs: map[string]interface{}{"item": 0},
			want: "none",
		},
		{
			name: "label non-zero value with format",
			text: `{{item|label "flag="|%05x}}`,
			subs: map[string]interface{}{"item": 5},
			want: "flag=00005",
		},
		{
			name: "label zero value with format",
			text: `{{item|label "flag="|%05x}}`,
			subs: map[string]interface{}{"item": 0},
			want: "00000",
		},
		{
			name: "label non-zero value",
			text: `{{item|label "flag="}}`,
			subs: map[string]interface{}{"item": 33},
			want: "flag=33",
		},
		{
			name: "left-justify string sub",
			text: "{{item|%-10s}}",
			subs: map[string]interface{}{"item": "test"},
			want: "test      ",
		},
		{
			name: "right-justify string sub",
			text: "{{item|%10s}}",
			subs: map[string]interface{}{"item": "test"},
			want: "      test",
		},
		{
			name: "simple string sub",
			text: "{{item}}",
			subs: map[string]interface{}{"item": "test"},
			want: "test",
		},
		{
			name: "simple boolean sub",
			text: "{{item}}",
			subs: map[string]interface{}{"item": false},
			want: "false",
		},
		{
			name: "simple array sub",
			text: "{{item}}",
			subs: map[string]interface{}{"item": []string{"one", "two"}},
			want: "[one two]",
		},
		{
			name: "no sub",
			text: "This is a simple string",
			want: "This is a simple string",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := handleFormat(tt.text, tt.subs); got != tt.want {
				t.Errorf("handleSub() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_handleSubMap(t *testing.T) {
	tests := []struct {
		name string
		text string
		subs map[string]interface{}
		want string
	}{
		{
			name: "complex case 1",
			text: `{{method}} {{endpoint}} {{file}} {{admin|empty|nonempty admin}} {{auth|empty|nonempty auth}}{{perms|label permissions=}}`,
			subs: map[string]interface{}{
				"method":   "GET",
				"endpoint": "/service/proc/Accounts",
				"file":     "accounts.go",
				"admin":    true,
				"auth":     false,
				"perms":    []string{"read", "write"},
			},
			want: `GET /service/proc/Accounts accounts.go admin permissions=[read write]`,
		},
		{
			name: "complex case 2",
			text: `{{method}} {{endpoint}} {{file}} {{admin|empty|nonempty admin}}{{auth|empty|nonempty auth}}{{perms|label permissions=}}`,
			subs: map[string]interface{}{
				"method":   "GET",
				"endpoint": "/service/proc/Accounts",
				"file":     "accounts.go",
				"admin":    false,
				"auth":     true,
				"perms":    []string{},
			},
			want: `GET /service/proc/Accounts accounts.go auth`,
		},
		{
			name: "no subs",
			text: "this is a test",
			want: "this is a test",
		},
		{
			name: "one sub",
			text: "this is a {{kind}} string",
			subs: map[string]interface{}{"kind": "test"},
			want: "this is a test string",
		},
		{
			name: "multiple subs",
			text: "this is a {{kind}} {{item}}",
			subs: map[string]interface{}{
				"kind": "test",
				"item": "value"},
			want: "this is a test value",
		},
		{
			name: "missing sub",
			text: "this is a {{kind}} {{item}}",
			subs: map[string]interface{}{
				"item": "value"},
			want: "this is a !kind! value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := handleSubstitutionMap(tt.text, tt.subs); got != tt.want {
				t.Errorf("handleSubMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_normalizeNumericValues(t *testing.T) {
	tests := []struct {
		name string
		arg  interface{}
		want interface{}
	}{
		{
			name: "float32",
			arg:  float32(123.456),
			want: float64(float32(123.456)),
		},
		{
			name: "float64",
			arg:  1.1,
			want: 1.1,
		},
		{
			name: "float64 to int",
			arg:  float64(-5.0),
			want: int(-5),
		},
		{
			name: "float32 to int",
			arg:  float32(123.000),
			want: int(123),
		},
		{
			name: "int",
			arg:  123,
			want: 123,
		},
		{
			name: "int32",
			arg:  int32(123),
			want: int(123),
		},
		{
			name: "string",
			arg:  "123",
			want: "123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeNumericValues(tt.arg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("normalizeNumericValues() = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}
