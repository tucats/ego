package i18n

import (
	"os"
	"sync"
	"testing"

	"github.com/tucats/ego/defs"
)

// resetDefaultLanguageForTesting clears the cached default-language state
// (both the Language variable itself and the sync.Once that guards
// deriving it from the environment) so that the next call to
// DefaultLanguage() re-derives it from the environment variables, instead
// of returning whatever an earlier test already computed and cached.
//
// This exists only so that this package's own tests can be run in any
// order and don't leak state into each other through the shared Language
// variable -- production code should never need anything like this, since
// the whole point of DefaultLanguage's sync.Once is that the default
// language is only ever supposed to be determined once per process.
func resetDefaultLanguageForTesting() {
	Language = ""
	defaultLanguageOnce = sync.Once{}
}

func TestT(t *testing.T) {
	resetDefaultLanguageForTesting()
	// Set up test cases
	tests := []struct {
		name      string
		key       string
		valueMap  map[string]any
		want      string
		wantError bool
	}{
		{
			name: "test with valid key and no valueMap",
			key:  "ego.hello",
			want: "Hello, {{name}}!",
		},
		{
			name: "test with valid key and valueMap",
			key:  "ego.hello",
			valueMap: map[string]any{
				"name": "Tom",
			},
			want: "Hello, Tom!",
		},
		{
			name:      "test with invalid key",
			key:       "invalid",
			want:      "invalid",
			wantError: true,
		},
	}

	// Set up environment variables
	os.Setenv(defs.EgoLangEnv, "en")

	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := T(tt.key, tt.valueMap)
			if got != tt.want {
				t.Errorf("T(%s) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

// TestTLang checks that TLang translates into whichever language it is
// explicitly given, regardless of what the process-default Language
// variable is currently set to -- this is the behavior that makes TLang
// safe to use for a single REST request that wants a different language
// than the server's own default.
func TestTLang(t *testing.T) {
	resetDefaultLanguageForTesting()

	// Deliberately set the process default to English, then prove that
	// TLang can still produce Spanish and French output for the very same
	// key. If TLang accidentally read the global Language variable instead
	// of its own lang parameter, this test would fail because every call
	// would return the English text.
	Language = "en"

	tests := []struct {
		name string
		lang string
		key  string
		want string
	}{
		{name: "english", lang: "en", key: "ego.hello", want: "Hello, {{name}}!"},
		{name: "spanish", lang: "es", key: "ego.hello", want: "¡Hola, {{name}}!"},
		{name: "french", lang: "fr", key: "ego.hello", want: "Bonjour, {{name}} !"},
		{
			// A language we have no catalog entry for at all should fall
			// back to English, exactly like T() falling back when the
			// process-default language has no translation.
			name: "unsupported language falls back to english",
			lang: "de",
			key:  "ego.hello",
			want: "Hello, {{name}}!",
		},
		{
			// A key with no translation in any language at all should
			// fall back to returning the key itself, so a missing
			// translation is obvious rather than silently blank.
			name: "unknown key falls back to the key itself",
			lang: "fr",
			key:  "no.such.key.exists",
			want: "no.such.key.exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Text(tt.lang, tt.key)
			if got != tt.want {
				t.Errorf("TLang(%q, %q) = %q, want %q", tt.lang, tt.key, got, tt.want)
			}
		})
	}

	// Confirm the calls above never modified the shared Language variable
	// -- TLang must be a pure function of its own arguments.
	if Language != "en" {
		t.Errorf("TLang calls modified the shared Language variable: got %q, want %q", Language, "en")
	}
}

// TestLLangMLangELang checks the "Lang" siblings of L, M, and E: each
// should translate into the given language and find entries under the
// "label.", "msg.", and "error." prefixes respectively, with the prefix
// itself never showing up in the returned text.
func TestLLangMLangELang(t *testing.T) {
	resetDefaultLanguageForTesting()

	t.Run("LLang", func(t *testing.T) {
		tests := []struct {
			lang string
			want string
		}{
			{lang: "en", want: "Name"},
			{lang: "es", want: "Nombre"},
			{lang: "fr", want: "Nom"},
		}

		for _, tt := range tests {
			if got := LLang(tt.lang, "Name"); got != tt.want {
				t.Errorf("LLang(%q, %q) = %q, want %q", tt.lang, "Name", got, tt.want)
			}
		}
	})

	t.Run("MLang", func(t *testing.T) {
		tests := []struct {
			lang string
			want string
		}{
			{lang: "en", want: "No rows in result"},
			{lang: "es", want: "No hay filas en el resultado"},
			{lang: "fr", want: "Aucune ligne dans le résultat"},
		}

		for _, tt := range tests {
			if got := MLang(tt.lang, "table.empty.rowset"); got != tt.want {
				t.Errorf("MLang(%q, %q) = %q, want %q", tt.lang, "table.empty.rowset", got, tt.want)
			}
		}
	})

	t.Run("ELang", func(t *testing.T) {
		tests := []struct {
			lang string
			want string
		}{
			{lang: "en", want: "User does not have read permission"},
			{lang: "es", want: "El usuario no tiene permiso de lectura"},
			{lang: "fr", want: "L'utilisateur n'a pas les autorisations de lecture"},
		}

		for _, tt := range tests {
			if got := ELang(tt.lang, "perm.read"); got != tt.want {
				t.Errorf("ELang(%q, %q) = %q, want %q", tt.lang, "perm.read", got, tt.want)
			}
		}
	})

	t.Run("missing key falls back to the unprefixed key, not the prefixed one", func(t *testing.T) {
		// translate()'s fallback-of-last-resort returns the exact key it
		// was asked to look up, which inside ofTypeLang is the PREFIXED
		// key (e.g. "label.NoSuchLabel"). ofTypeLang must strip that
		// prefix back off before returning, so the caller sees their own
		// original "NoSuchLabel" rather than the internal "label."-prefixed
		// form.
		got := LLang("en", "NoSuchLabelExistsAnywhere")
		want := "NoSuchLabelExistsAnywhere"

		if got != want {
			t.Errorf("LLang with missing key = %q, want %q", got, want)
		}
	})
}

// TestDefaultLanguageIsRaceFree exercises DefaultLanguage from many
// goroutines at once, immediately after resetting its one-time
// initialization state. This is exactly the scenario that used to be
// unsafe before defaultLanguageOnce was introduced: many REST request
// goroutines all calling a translation function for the very first time
// at nearly the same moment. Run with "go test -race" to have the Go race
// detector actively confirm there is no data race; without -race this
// test still confirms every goroutine agrees on the same answer.
func TestDefaultLanguageIsRaceFree(t *testing.T) {
	resetDefaultLanguageForTesting()
	os.Setenv(defs.EgoLangEnv, "fr")

	const goroutineCount = 50

	results := make([]string, goroutineCount)

	var wg sync.WaitGroup

	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)

		go func(index int) {
			defer wg.Done()

			results[index] = DefaultLanguage()
		}(i)
	}

	wg.Wait()

	for i, got := range results {
		if got != "fr" {
			t.Errorf("goroutine %d: DefaultLanguage() = %q, want %q", i, got, "fr")
		}
	}
}
