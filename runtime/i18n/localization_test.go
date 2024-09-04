package i18n

import (
	"os"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

func TestTranslation(t *testing.T) {
	tests := []struct {
		name     string
		lang     string
		args     data.List
		expected string
	}{
		{
			name: "Translation with parameters",
			lang: "en",
			args: data.NewList(
				"welcome",
				data.NewMapFromMap(
					map[string]string{
						"name": "John",
					},
				),
			),
			expected: "Welcome, John!",
		},
		{
			name:     "Simple translation",
			lang:     "en",
			args:     data.NewList("hello"),
			expected: "hello",
		},
		{
			name:     "Simple translation with explicit language",
			lang:     "en",
			args:     data.NewList("hello", nil, "fr"),
			expected: "bonjour",
		},
		{
			name:     "Translation with different language",
			lang:     "fr",
			args:     data.NewList("hello"),
			expected: "bonjour",
		},
		{
			name:     "Translation with missing property",
			lang:     "en",
			args:     data.NewList("missing"),
			expected: "expecting, <no value>!",
		},
		{
			name:     "Translation with invalid argument type",
			args:     data.NewList(123),
			expected: "123",
		},
	}

	s := symbols.NewSymbolTable("test")

	m := data.NewStruct(data.StructType)
	_ = m.Set("en", data.NewStructFromMap(
		map[string]interface{}{
			"hello":   "hello",
			"welcome": "Welcome, {{.name}}!",
			"missing": "expecting, {{.name}}!",
		},
	))
	_ = m.Set("fr", data.NewStructFromMap(
		map[string]interface{}{
			"hello": "bonjour",
		},
	))

	s.SetAlways(defs.LocalizationVariable, m)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure the language is set for this test.
			os.Setenv("LANG", tt.lang)

			_, _ = language(s, data.NewList())

			got, err := translation(s, tt.args)
			if err != nil {
				t.Errorf("translation() error = %v", err)
			}
			
			if got != tt.expected {
				t.Errorf("translation() = %v, want %v", got, tt.expected)
			}
		})
	}
}
