package i18n

import (
	"sort"
	"strconv"
	"strings"
	"sync"
)

// supportedLanguagesMu, supportedLanguagesCache, and supportedLanguagesValid
// together implement a small cache for SupportedLanguages, below.
//
// Computing the answer to "what languages do we have translations for"
// means walking every single entry in the messages map (there are
// thousands of them), which is wasteful to redo on every REST request
// just to negotiate a language. So we compute it once and remember the
// answer, only recomputing it if the catalog actually changes (which
// happens when MergeLocalization loads additional translations — see the
// call to invalidateSupportedLanguagesCache in merge.go).
//
// We use a sync.RWMutex rather than a plain sync.Mutex because the common
// case — many goroutines just reading an already-computed cache — should
// not have to wait on each other. A RWMutex lets any number of readers in
// at once via RLock/RUnlock, and only blocks everyone when a writer needs
// the exclusive Lock/Unlock to actually change the cached value.
var (
	supportedLanguagesMu    sync.RWMutex
	supportedLanguagesCache []string
	supportedLanguagesValid bool
)

// SupportedLanguages returns the distinct language codes (for example
// "en", "es", "fr") that appear anywhere in Ego's message catalog. The
// result is sorted alphabetically and cached; see the comment on the
// package-level cache variables above for why.
func SupportedLanguages() []string {
	// Fast path: most calls will find the cache already populated, so try
	// a (shared, non-blocking-of-other-readers) read lock first.
	supportedLanguagesMu.RLock()

	if supportedLanguagesValid {
		cached := supportedLanguagesCache

		supportedLanguagesMu.RUnlock()

		return cached
	}

	supportedLanguagesMu.RUnlock()

	// Slow path: the cache is missing or was invalidated, so we need to
	// (re)compute it. This requires the exclusive write lock, since we're
	// about to change the shared cache variables.
	supportedLanguagesMu.Lock()
	defer supportedLanguagesMu.Unlock()

	// It's possible that another goroutine got here first and already
	// recomputed the cache while we were waiting for the write lock above
	// — if so, there's no need to redo that work.
	if supportedLanguagesValid {
		return supportedLanguagesCache
	}

	seen := map[string]bool{}

	for _, translationsForOneKey := range messages {
		for lang := range translationsForOneKey {
			seen[lang] = true
		}
	}

	result := make([]string, 0, len(seen))

	for lang := range seen {
		result = append(result, lang)
	}

	sort.Strings(result)

	supportedLanguagesCache = result
	supportedLanguagesValid = true

	return result
}

// invalidateSupportedLanguagesCache discards the cached result of
// SupportedLanguages, so that the next call recomputes it from whatever
// the messages map currently contains. This is called automatically from
// MergeLocalization (see merge.go) whenever new translations are merged
// into the catalog, so callers of SupportedLanguages never need to worry
// about it themselves.
func invalidateSupportedLanguagesCache() {
	supportedLanguagesMu.Lock()
	defer supportedLanguagesMu.Unlock()

	supportedLanguagesValid = false
}

// isSupportedLanguage reports whether lang is one of the language codes
// present in the message catalog.
func isSupportedLanguage(lang string) bool {
	for _, supported := range SupportedLanguages() {
		if supported == lang {
			return true
		}
	}

	return false
}

// NegotiateLanguage parses a raw HTTP "Accept-Language" header value —
// the same header a web browser sends to ask a website for a page in the
// user's preferred language — and returns the best-matching language code
// that Ego actually has translations for, such as "en", "es", or "fr".
//
// A typical header value looks like this:
//
//	fr-CA,fr;q=0.9,en;q=0.8,*;q=0.1
//
// which a browser sends to mean, in order of decreasing preference:
// French as spoken in Canada, then French in general, then English, then
// (with very low priority) anything else. The ";q=<number>" part is
// called a "quality value" and ranges from 0 (least preferred) to 1 (most
// preferred, and also the default when ";q=" is omitted entirely).
//
// If nothing in the header matches a language Ego supports — including
// the common case where header is empty, because the client didn't send
// the header at all — this function returns "". Callers should treat an
// empty result as "let the server pick its own default" (see
// DefaultLanguage) rather than as an error: omitting Accept-Language, or
// only asking for a language Ego has no catalog entries for, is normal,
// unremarkable client behavior, not something to reject.
//
// This is a deliberately simple parser, not a complete implementation of
// the language-negotiation algorithm described in RFC 7231 section 5.3.5.
// It honors quality values (so it picks the client's most-preferred
// supported language, not just the first one listed) but only matches on
// the primary language subtag — the part before the first hyphen, such as
// "fr" out of "fr-CA" — and does not attempt to also prefer a
// region-specific match over a generic one. Given that Ego currently ships
// only three languages (English, Spanish, and French), none of which Ego
// distinguishes by region, this simpler approach is sufficient; a fuller
// parser (or an existing third-party library) could replace this later if
// Ego ever needs to handle regional dialects differently.
func NegotiateLanguage(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}

	// Each candidate is one language tag from the header, paired with the
	// quality value the client assigned to it, so that we can later sort
	// them from most- to least-preferred.
	type candidate struct {
		lang    string
		quality float64
	}

	candidates := make([]candidate, 0)

	// An Accept-Language header is a comma-separated list of tags, each
	// optionally followed by ";q=<quality>". Split on commas first to get
	// each "tag;q=value" piece on its own.
	for _, rawTag := range strings.Split(header, ",") {
		rawTag = strings.TrimSpace(rawTag)
		if rawTag == "" {
			continue
		}

		// Now split that piece on semicolons: the first part is the
		// language tag itself, and any parts after that are parameters
		// (we only care about the "q=" one).
		parts := strings.Split(rawTag, ";")
		tag := strings.TrimSpace(parts[0])

		// A bare "*" means "any language is fine" — it doesn't tell us
		// anything about which of OUR supported languages the client
		// would actually prefer, so we skip it and let either a more
		// specific tag elsewhere in the header, or our own default
		// language, win instead.
		if tag == "" || tag == "*" {
			continue
		}

		// The default quality value, when ";q=" is not present at all,
		// is 1.0 — the highest possible preference.
		quality := 1.0

		for _, param := range parts[1:] {
			param = strings.TrimSpace(param)

			const qualityPrefix = "q="
			if !strings.HasPrefix(param, qualityPrefix) {
				continue
			}

			parsedQuality, err := strconv.ParseFloat(strings.TrimPrefix(param, qualityPrefix), 64)
			if err == nil {
				quality = parsedQuality
			}
			// If the quality value doesn't parse as a number, we simply
			// keep the default of 1.0 rather than rejecting the whole
			// header — a slightly malformed quality value from a client
			// shouldn't prevent language negotiation from working at all.
		}

		// Language tags can include a region, such as "fr-CA" (French as
		// spoken in Canada) or "en-GB" (English as spoken in the UK).
		// Ego's catalog only distinguishes translations by the primary
		// language code ("fr", "en"), so we only keep the part before the
		// first hyphen, and lower-case it to match how the catalog stores
		// its keys (see SupportedLanguages).
		primaryLang := tag
		if dash := strings.IndexByte(primaryLang, '-'); dash >= 0 {
			primaryLang = primaryLang[:dash]
		}

		primaryLang = strings.ToLower(primaryLang)
		if primaryLang == "" {
			continue
		}

		candidates = append(candidates, candidate{lang: primaryLang, quality: quality})
	}

	// Sort the candidates from most- to least-preferred. sort.SliceStable
	// (rather than plain sort.Slice) preserves the original left-to-right
	// order of any two candidates that have the same quality value, which
	// matches how a client lists multiple equally preferred languages.
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].quality > candidates[j].quality
	})

	// Walk the candidates in preference order and return the first one
	// Ego actually has a translation catalog for.
	for _, c := range candidates {
		if isSupportedLanguage(c.lang) {
			return c.lang
		}
	}

	// Nothing in the header matched a language we support.
	return ""
}
