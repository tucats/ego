package strconv

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func TestDoQuote(t *testing.T) {
	tests := []struct {
		name     string
		input    data.List
		expected string
	}{
		{
			name:     "Empty string",
			input:    data.NewList(""),
			expected: `""`,
		},
		{
			name:     "String with spaces",
			input:    data.NewList("Hello World"),
			expected: `"Hello World"`,
		},
		{
			name:     "String with special characters",
			input:    data.NewList("Hello, World!"),
			expected: `"Hello, World!"`,
		},
		{
			name:     "String with quotes",
			input:    data.NewList(`"Hello, World!"`),
			expected: `"\"Hello, World!\""`,
		},
		{
			name:     "String with backslashes",
			input:    data.NewList(`\Hello, World!\`),
			expected: `"\\Hello, World!\\"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewSymbolTable("testing")
			got, err := doQuote(s, tt.input)
			if err != nil {
				t.Errorf("doQuote() error = %v", err)
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("doQuote() got = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDoUnquote_InvalidQuotes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "No quotes",
			input:    "Hello, World!",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Single quote at the beginning",
			input:    "'Hello, World!",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Single quote at the end",
			input:    "Hello, World!'",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Double quote at the beginning",
			input:    "\"Hello, World!",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Double quote at the end",
			input:    "Hello, World!\"",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := symbols.NewSymbolTable("testing")
			args := data.NewList(tt.input)
			got, err := doUnquote(s, args)

			if (err != nil) != tt.wantErr {
				t.Errorf("doUnquote() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				list, ok := got.(data.List)
				if !ok {
					t.Errorf("doUnquote() got = %v, want data.List", got)
					return
				}

				str, ok := list.Get(0).(string)
				if !ok {
					t.Errorf("doUnquote() got = %v, want string", list.Get(0))
					return
				}

				if str != tt.expected {
					t.Errorf("doUnquote() got = %v, want %v", str, tt.expected)
				}
			}
		})
	}
}
