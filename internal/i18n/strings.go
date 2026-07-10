// Package i18n provides localization and internationalization
// functionality for Ego itself.
package i18n

// This generates the messages.go file that contains the messages map,
// using the language files
//go:generate go run ../../tools/lang/ -c -p languages -s messages.go

import (
	"os"
	"strings"
	"sync"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/subs"
)

// Language is a string that identifies the process-wide default language,
// such as "en" for English or "fr" for French. It is used as a key into
// the message catalog (the "messages" map generated into messages.go) by
// the plain T, L, M, and E functions below.
//
// IMPORTANT — read this before touching this variable: it is a single,
// shared, package-level variable, so it is only safe to read or write it
// from code that runs "single-threaded", such as the command-line startup
// sequence (see app-cli/app/actions.go's LanguageAction, which sets this
// from the --language option) or a unit test. It is NOT safe to assign to
// this variable from REST request-handling code, because many requests
// can be processed concurrently on different goroutines — two goroutines
// racing to write different language codes into the same variable at the
// same time is exactly the kind of bug Go's race detector exists to catch.
//
// If a single REST request needs a response in a language that might be
// different from the process default — for example, because the caller
// sent an "Accept-Language" header — do NOT change this variable. Instead,
// use one of the "Lang" suffixed functions below (TLang, LLang, MLang,
// ELang). Each of those takes the desired language as an explicit
// argument and never reads or writes this variable, so they can safely be
// called concurrently, from any number of goroutines, even passing
// different language values at the exact same time.
var Language string

// defaultLanguageOnce guards the one-time work of computing a default
// value for Language from the process environment (see DefaultLanguage
// below). Using sync.Once here, instead of the simple "if Language == ”"
// check this code used before, closes a real data race: without it, two
// goroutines could both call a translation function for the very first
// time at nearly the same moment, both see that Language is still empty,
// and both try to compute and write a value into it at once. sync.Once
// guarantees that the work inside its Do function happens exactly once,
// no matter how many goroutines call it concurrently — every caller
// blocks until the first one finishes, and only the first one actually
// runs the initialization code.
var defaultLanguageOnce sync.Once

// DefaultLanguage returns the process-wide default language code, such as
// "en" or "fr". This is the language that T, L, M, and E (the
// language-agnostic translation functions below) use.
//
// The very first time this function is called, from any goroutine, it
// lazily figures out the default language by checking the EGO_LANG
// environment variable, then the more generic LANG environment variable,
// exactly like the original implementation of this package did. Every
// call after that first one just returns whatever Language currently
// holds — which means code that explicitly assigns to Language (such as
// the CLI's --language option, handled before any translation has
// happened) continues to take priority over the environment variables,
// exactly as before.
func DefaultLanguage() string {
	defaultLanguageOnce.Do(func() {
		// Only derive a value from the environment if nothing has already
		// assigned a non-empty value to Language. This preserves the
		// original behavior, where an explicit assignment (for example by
		// the CLI's --language option) wins over the environment
		// variables, as long as that assignment happens before the first
		// translation call.
		if Language == "" {
			Language = os.Getenv(defs.EgoLangEnv)
			if Language == "" {
				Language = os.Getenv("LANG")
			}

			// Environment variables often contain a longer locale
			// string, such as "en_US.UTF-8" or "fr_FR" — we only care
			// about the two-letter language code at the very front of
			// that string, since that's how the message catalog is
			// keyed.
			if len(Language) > 2 {
				Language = Language[0:2]
			}
		}
	})

	return Language
}

// translate is the shared lookup logic behind both T (which always uses
// the process default language) and TLang (which uses an explicit
// language passed in by the caller). Separating this out means the two
// public functions can share one implementation while only one of them
// ever touches the shared Language variable.
//
// translate only ever READS from the messages map — by the time the
// server is handling requests, nothing should be writing to that map
// anymore (see merge.go for the one place that does write to it, which
// only happens during single-threaded command-line startup). Because of
// that, this function never needs any locking of its own: concurrent
// reads of a Go map from multiple goroutines are safe, as long as nothing
// is writing to it at the same time.
//
// If key has no translation in lang, this falls back to the English
// ("en") translation. If there's no English translation either, it falls
// back to returning key itself, so that a missing translation is obvious
// in the output (you'll see the raw key, like "error.some.key", rather
// than a blank string).
func translate(lang, key string, valueMap ...map[string]any) string {
	text, ok := messages[key][lang]
	if !ok {
		text, ok = messages[key]["en"]
		if !ok {
			text = key
		}
	}

	// The valueMap is an optional argument, but there can be only zero or
	// one of them — having more than one wouldn't make sense, since
	// there's only one string to substitute values into.
	if len(valueMap) > 1 {
		return "@@@ INCORRECT USAGE WITH MULTIPLE VALUE MAPS @@@"
	}

	if len(valueMap) > 0 {
		text, _ = subs.SubstituteMap(text, valueMap[0])
	}

	return text
}

// T returns the translated string for the given key, using the process
// default language (see DefaultLanguage). If the optional valueMap is
// provided, any "{{tag}}" placeholders in the translated string are
// replaced with the corresponding value from the map.
//
// T (and L, M, E below) are meant for code that only ever needs to speak
// one language at a time for the whole process — which describes almost
// all of Ego's command-line code, since a single CLI invocation has a
// single user with a single language preference. REST server code that
// must support different languages for different, simultaneous requests
// (for example, honoring each caller's own Accept-Language header) should
// use TLang instead — see below.
func T(key string, valueMap ...map[string]any) string {
	return translate(DefaultLanguage(), key, valueMap...)
}

// Text behaves exactly like T, except that it always translates into the
// lang passed in by the caller ("en", "fr", "es", and so on) instead of
// the process default language. It never reads or writes the shared
// Language variable, so it is safe to call concurrently: for instance,
// one REST request's handler could call Text("fr", ...) while, at the
// very same moment on a different goroutine, another request's handler
// calls Text("es", ...) for a completely different caller — neither call
// affects the other.
func Text(lang, key string, valueMap ...map[string]any) string {
	return translate(lang, key, valueMap...)
}

// ofTypeLang is the shared implementation behind L, M, E, and their "Lang"
// siblings (LLang, MLang, ELang). Ego's message catalog groups every
// string under one of a few well-known prefixes — "label.", "msg.", or
// "error." — so that, for example, the word "Name" used as a table column
// heading (catalog key "label.Name") can have a different translation
// than the same English word used inside a sentence elsewhere (which
// would live under a "msg." key instead). The prefix parameter says which
// of those groups to look in.
//
// translate's fallback-of-last-resort, when a key has no translation at
// all, is to return the exact key it was given — which here would be the
// prefixed key, like "label.SomeUntranslatedThing". Since the caller only
// ever supplied the unprefixed part ("SomeUntranslatedThing"), we strip
// the prefix back off of that fallback value before returning it, so a
// caller who forgets to add a catalog entry still gets back their own
// original key text instead of a mildly confusing prefixed version of it.
func ofTypeLang(lang, prefix, key string, valueMap ...map[string]any) string {
	prefixWithDot := prefix + "."
	translated := translate(lang, prefixWithDot+key, valueMap...)

	return strings.TrimPrefix(translated, prefixWithDot)
}

// ofType is the process-default-language version of ofTypeLang; see L, M,
// and E below for the three supported prefixes.
func ofType(prefix, key string, valueMap ...map[string]any) string {
	return ofTypeLang(DefaultLanguage(), prefix, key, valueMap...)
}

// L returns a label with the given key, translated into the process
// default language. Labels are short pieces of text used as headings or
// identifiers, such as table column names ("Name", "Value", and so on).
func L(key string, valueMap ...map[string]any) string {
	return ofType("label", key, valueMap...)
}

// LLang behaves like L, but always translates into the lang passed in by
// the caller instead of the process default language. As with TLang, this
// never touches the shared Language variable, so it is safe to call
// concurrently with different lang values from different goroutines.
func LLang(lang, key string, valueMap ...map[string]any) string {
	return ofTypeLang(lang, "label", key, valueMap...)
}

// M returns a general informational message with the given key,
// translated into the process default language. Messages are
// general-purpose text shown to the user, distinct from short labels (see
// L) or error text (see E).
func M(key string, valueMap ...map[string]any) string {
	return ofType("msg", key, valueMap...)
}

// MLang behaves like M, but always translates into the lang passed in by
// the caller instead of the process default language.
func MLang(lang, key string, valueMap ...map[string]any) string {
	return ofTypeLang(lang, "msg", key, valueMap...)
}

// E returns an error message with the given key, translated into the
// process default language.
func E(key string, valueMap ...map[string]any) string {
	return ofType("error", key, valueMap...)
}

// ELang behaves like E, but always translates into the lang passed in by
// the caller instead of the process default language.
func ELang(lang, key string, valueMap ...map[string]any) string {
	return ofTypeLang(lang, "error", key, valueMap...)
}
