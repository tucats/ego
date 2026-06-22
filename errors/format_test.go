package errors

import (
	"strings"
	"testing"

	"github.com/tucats/ego/i18n"
)

// TestError_Localize checks that Localize renders the same underlying
// error in whatever language it is asked for, independently of whatever
// the process-wide default language (i18n.Language) happens to be set to.
//
// "perm.read" is used here because it is a real key in Ego's message
// catalog (error.perm.read) with English, Spanish, and French
// translations already defined -- see i18n/messages.go -- so this test
// exercises the real catalog rather than a synthetic one.
func TestError_Localize(t *testing.T) {
	// Deliberately set the process default language to English. The
	// whole point of this test is to prove that Localize does NOT depend
	// on this value, so if a future change accidentally made Localize
	// read the global Language variable instead of its own lang
	// parameter, this test would fail as soon as a non-English lang is
	// requested.
	i18n.Language = "en"

	e := Message("perm.read")

	tests := []struct {
		name string
		lang string
		want string
	}{
		{name: "english", lang: "en", want: "User does not have read permission"},
		{name: "spanish", lang: "es", want: "El usuario no tiene permiso de lectura"},
		{name: "french", lang: "fr", want: "L'utilisateur n'a pas les autorisations de lecture"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := e.Localize(tt.lang); got != tt.want {
				t.Errorf("Localize(%q) = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}

// TestError_ErrorUsesDefaultLanguage checks that the plain Error() method
// (the one that satisfies Go's standard "error" interface) keeps behaving
// the way it always did: it renders using the process-wide default
// language, i18n.Language, rather than requiring a lang argument the way
// Localize does.
func TestError_ErrorUsesDefaultLanguage(t *testing.T) {
	e := Message("perm.read")

	i18n.Language = "es"
	if got, want := e.Error(), "El usuario no tiene permiso de lectura"; got != want {
		t.Errorf("Error() with Language=es = %q, want %q", got, want)
	}

	i18n.Language = "fr"
	if got, want := e.Error(), "L'utilisateur n'a pas les autorisations de lecture"; got != want {
		t.Errorf("Error() with Language=fr = %q, want %q", got, want)
	}

	// Restore English so this test doesn't leak a non-English default
	// language into whichever test happens to run next in this package.
	i18n.Language = "en"
}

// TestError_LocalizeChainedError checks that Localize threads its lang
// argument all the way through a chain of linked errors (built with
// Chain), rather than only translating the outermost error and leaving
// any chained errors in the process-default language.
func TestError_LocalizeChainedError(t *testing.T) {
	i18n.Language = "en"

	outer := Message("perm.read")
	inner := Message("perm.write")

	outer.Chain(inner)

	got := outer.Localize("fr")

	wantOuter := "L'utilisateur n'a pas les autorisations de lecture"
	wantInner := "L'utilisateur n'a pas les autorisations d'écriture"

	if !strings.Contains(got, wantOuter) {
		t.Errorf("Localize(%q) = %q, expected it to contain the outer French translation %q", "fr", got, wantOuter)
	}

	if !strings.Contains(got, wantInner) {
		t.Errorf("Localize(%q) = %q, expected it to contain the chained French translation %q", "fr", got, wantInner)
	}
}
