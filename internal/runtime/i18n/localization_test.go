package i18n

import (
	"testing"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/symbols"

	EgoLocalization "github.com/tucats/ego/internal/i18n"
)

// unwrapValue extracts index 0 from a data.List result. Functions that follow
// the (value, error) convention return a data.List{value, err}, so their
// Go-level "any" result must be unwrapped before use.
func unwrapValue(t *testing.T, result any) any {
	t.Helper()

	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("expected data.List result, got %T", result)
	}

	return list.Get(0)
}

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
			expected: "expecting, {{name}}!",
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
		map[string]any{
			"hello":   "hello",
			"welcome": "Welcome, {{name}}!",
			"missing": "expecting, {{name}}!",
		},
	))
	_ = m.Set("fr", data.NewStructFromMap(
		map[string]any{
			"hello": "bonjour",
		},
	))

	s.SetAlways(defs.LocalizationVariable, m)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure the language is set for this test. Set the shared
			// package var directly rather than the LANG environment
			// variable: DefaultLanguage() only ever derives a value from
			// the environment once per process (sync.Once), so an env var
			// change wouldn't be picked up by a later sub-test, but it
			// always returns the *current* value of Language itself.
			EgoLocalization.Language = tt.lang

			_, _ = language(s, data.NewList())

			rawGot, err := translation(s, tt.args)
			if err != nil {
				t.Errorf("translation() error = %v", err)
			}

			got := unwrapValue(t, rawGot)

			if got != tt.expected {
				t.Errorf("translation() = %v, want %v", got, tt.expected)
			}
		})
	}
}
