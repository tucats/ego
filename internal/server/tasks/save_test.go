package tasks

import "testing"

func resetSaved(t *testing.T) {
	t.Helper()

	saveLock.Lock()
	saved = map[string]string{}
	saveLock.Unlock()
}

func TestSubstituteReplacesKnownToken(t *testing.T) {
	resetSaved(t)
	setSaved("TOKEN", "abc123")

	got := substitute(`{"auth": "{{TOKEN}}"}`)
	want := `{"auth": "abc123"}`

	if got != want {
		t.Errorf("substitute() = %q, want %q", got, want)
	}
}

func TestSubstituteLeavesUnknownTokenUnchanged(t *testing.T) {
	resetSaved(t)

	text := `{"auth": "{{NOPE}}"}`

	if got := substitute(text); got != text {
		t.Errorf("substitute() = %q, want unchanged %q", got, text)
	}
}

func TestSubstituteHandlesMultipleTokens(t *testing.T) {
	resetSaved(t)
	setSaved("A", "1")
	setSaved("B", "2")

	got := substitute("{{A}}-{{B}}-{{A}}")
	want := "1-2-1"

	if got != want {
		t.Errorf("substitute() = %q, want %q", got, want)
	}
}

func TestSubstituteNoTokensIsUnchanged(t *testing.T) {
	resetSaved(t)

	text := "plain text with no braces"

	if got := substitute(text); got != text {
		t.Errorf("substitute() = %q, want unchanged %q", got, text)
	}
}

func TestSetSavedOverwritesExistingValue(t *testing.T) {
	resetSaved(t)
	setSaved("X", "first")
	setSaved("X", "second")

	if got := substitute("{{X}}"); got != "second" {
		t.Errorf("substitute() = %q, want %q", got, "second")
	}
}
