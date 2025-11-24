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
		subs map[string]any
		want string
	}{
		{
			name: "magnitude with value 1",
			text: `{{value|zero "frog free"|one frog|many frogs}}`,
			subs: map[string]any{"value": 1},
			want: "frog",
		},
		{
			name: "magnitude with value 0",
			text: `{{value|zero "frog free"|one frog|many frogs}}`,
			subs: map[string]any{"value": 0},
			want: "frog free",
		},
		{
			name: "magnitude with value 33",
			text: `{{value|zero "frog free"|one frog|many frogs}}`,
			subs: map[string]any{"value": 33},
			want: "frogs",
		},
		{
			name: "left",
			text: "{{value|left 8}}",
			subs: map[string]any{"value": "abc"},
			want: "abc     ",
		},
		{
			name: "center",
			text: "{{value|center 8}}",
			subs: map[string]any{"value": "abc"},
			want: "  abc   ",
		},
		{
			name: "right",
			text: "{{value|right 8}}",
			subs: map[string]any{"value": "abc"},
			want: "     abc",
		},
		{
			name: "format with center",
			text: "{{value|%3.1f|center 8}}",
			subs: map[string]any{"value": 5.6},
			want: "  5.6   ",
		},
		{
			name: "combo format and list",
			text: "{{value|%02d|list}}",
			subs: map[string]any{"value": []any{1, 2, 3}},
			want: "01, 02, 03",
		},
		{
			name: "combo format, list, and size",
			text: "{{value|%02d|list|size 8}}",
			subs: map[string]any{"value": []any{1, 2, 3}},
			want: "01, 0...",
		},
		{
			name: "combo list and size truncated",
			text: "{{value|list|size 10}}",
			subs: map[string]any{"value": []string{"one", "two", "three"}},
			want: "one, tw...",
		},
		{
			name: "combo list and size not truncated",
			text: "{{value|list|size 20}}",
			subs: map[string]any{"value": []string{"one", "two", "three"}},
			want: "one, two, three",
		},
		{
			name: "size not needed",
			text: `{{value|size 10}}`,
			subs: map[string]any{"value": "test"},
			want: "test",
		},
		{
			name: "size needed",
			text: `{{value|size 10}}`,
			subs: map[string]any{"value": "test string of text"},
			want: "test st...",
		},
		{
			name: "size invalid",
			text: `{{value|size 2}}`,
			subs: map[string]any{"value": "test string of text"},
			want: "!Invalid size: 2!",
		},
		{
			name: "simple pad",
			text: `{{size|pad "*"}}`,
			subs: map[string]any{"size": 3},
			want: "***",
		},
		{
			name: "complex pad",
			text: `{{size|pad "XO"}}`,
			subs: map[string]any{"size": 2},
			want: "XOXO",
		},
		{
			name: "zero pad",
			text: `{{size|pad "*"}}`,
			subs: map[string]any{"size": 0},
			want: "",
		},
		{
			name: "label zero value",
			text: `{{item|label "flag="}}`,
			subs: map[string]any{"item": 0},
			want: "",
		},
		{
			name: "explicit format string sub",
			text: "{{item|format %10s}}",
			subs: map[string]any{"item": "test"},
			want: "      test",
		},
		{
			name: "label zero value with empty value",
			text: `{{item|label "flag="|%05x | empty none}}`,
			subs: map[string]any{"item": 0},
			want: "none",
		},
		{
			name: "label non-zero value with format",
			text: `{{item|label "flag="|%05x}}`,
			subs: map[string]any{"item": 5},
			want: "flag=00005",
		},
		{
			name: "label zero value with format",
			text: `{{item|label "flag="|%05x}}`,
			subs: map[string]any{"item": 0},
			want: "00000",
		},
		{
			name: "label non-zero value",
			text: `{{item|label "flag="}}`,
			subs: map[string]any{"item": 33},
			want: "flag=33",
		},
		{
			name: "left-justify string sub",
			text: "{{item|%-10s}}",
			subs: map[string]any{"item": "test"},
			want: "test      ",
		},
		{
			name: "right-justify string sub",
			text: "{{item|%10s}}",
			subs: map[string]any{"item": "test"},
			want: "      test",
		},
		{
			name: "simple string sub",
			text: "{{item}}",
			subs: map[string]any{"item": "test"},
			want: "test",
		},
		{
			name: "simple boolean sub",
			text: "{{item}}",
			subs: map[string]any{"item": false},
			want: "false",
		},
		{
			name: "simple array sub",
			text: "{{item}}",
			subs: map[string]any{"item": []string{"one", "two"}},
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
		subs map[string]any
		want string
	}{
		{
			name: "floating format",
			text: "Garbage collection pct of cpu:     {{cpu|%8.7f}}",
			subs: map[string]any{
				"cpu": 1.2,
			},
			want: "Garbage collection pct of cpu:     1.2000000",
		},
		{
			name: "using pad",
			text: `{{addr|%4d}}: {{depth|pad "| "}}{{op}} {{operand}}`,
			subs: map[string]any{
				"addr":    1234,
				"depth":   3,
				"op":      "LOAD_FAST",
				"operand": "42",
			},
			want: `1234: | | | LOAD_FAST 42`,
		},
		{
			name: "complex case 1",
			text: `{{method}} {{endpoint}} {{file}} {{admin|empty|nonempty admin}} {{auth|empty|nonempty auth}}{{perms|label permissions=}}`,
			subs: map[string]any{
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
			subs: map[string]any{
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
			subs: map[string]any{"kind": "test"},
			want: "this is a test string",
		},
		{
			name: "multiple subs",
			text: "this is a {{kind}} {{item}}",
			subs: map[string]any{
				"kind": "test",
				"item": "value"},
			want: "this is a test value",
		},
		{
			name: "missing sub",
			text: "this is a {{kind}} {{item}}",
			subs: map[string]any{
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
		arg  any
		want any
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
			if got := normalizeNumericValues(tt.arg, false); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("normalizeNumericValues() = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func Test_barEscape(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{
			name: "quoted sub",
			arg:  `one "|" two`,
			want: `one "!BAR!" two`,
		},
		{
			name: "multiple quoted sub",
			arg:  `one "|" two "|"`,
			want: `one "!BAR!" two "!BAR!"`,
		},
		{
			name: "mixed quoted sub",
			arg:  `one "|" two | three`,
			want: `one "!BAR!" two | three`,
		},
		{
			name: "no sub",
			arg:  "this is a simple string",
			want: "this is a simple string",
		},
		{
			name: "single sub",
			arg:  "one|two",
			want: "one|two",
		},
		{
			name: "multiple sub",
			arg:  "one|two|three",
			want: "one|two|three",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := barEscape(tt.arg); got != tt.want {
				t.Errorf("barEscape() = %v, want %v", got, tt.want)
			}
		})
	}
}
