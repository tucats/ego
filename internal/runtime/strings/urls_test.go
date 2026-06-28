package strings

import (
	"reflect"
	"testing"
)

func TestParseURLPattern(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		pattern string
		want    map[string]any
		matches bool
	}{
		{
			name:    "constant pattern unused segment",
			url:     "/service/debug",
			pattern: "/service/debug/age",
			want: map[string]any{
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
			want: map[string]any{
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
			want: map[string]any{
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
			want: map[string]any{
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
			want: map[string]any{
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
			want: map[string]any{
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
