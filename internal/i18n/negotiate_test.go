package i18n

import "testing"

// containsString is a small test helper that reports whether target is
// present anywhere in list. (Go's standard library didn't have a generic
// "contains" helper for strings until slices.Contains was added in Go
// 1.21, and this keeps the test self-contained either way.)
func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}

	return false
}

// TestSupportedLanguages checks that the well-known languages Ego ships
// translations for today (English, Spanish, French) are reported by
// SupportedLanguages. It deliberately does not assert the EXACT contents
// of the returned slice, since other tests in this package (and the
// production message catalog itself, over time) may add more languages.
func TestSupportedLanguages(t *testing.T) {
	got := SupportedLanguages()

	for _, want := range []string{"en", "es", "fr"} {
		if !containsString(got, want) {
			t.Errorf("SupportedLanguages() = %v, missing expected language %q", got, want)
		}
	}
}

// TestSupportedLanguagesCacheInvalidation confirms that merging in a new
// localization (as MergeLocalization does, for example when an operator
// loads a --localization-file at startup) is reflected the next time
// SupportedLanguages is called, even if SupportedLanguages had already
// been called -- and therefore cached its answer -- before the merge
// happened. This is the behavior that invalidateSupportedLanguagesCache
// (called from MergeLocalization) exists to guarantee.
func TestSupportedLanguagesCacheInvalidation(t *testing.T) {
	// Prime the cache with whatever the catalog looks like right now,
	// before we add a language nothing else in the catalog has.
	before := SupportedLanguages()
	if containsString(before, "zz") {
		t.Fatalf("test setup problem: %q is already a supported language, pick a different fake code", "zz")
	}

	count := MergeLocalization(map[string]map[string]string{
		"test.only.negotiate.cache.key": {"zz": "nur ein Test (fake)"},
	})

	if count != 1 {
		t.Fatalf("MergeLocalization() = %d, want 1", count)
	}

	after := SupportedLanguages()
	if !containsString(after, "zz") {
		t.Errorf("SupportedLanguages() after merge = %v, expected it to now include %q", after, "zz")
	}
}

// TestNegotiateLanguage exercises NegotiateLanguage against a variety of
// Accept-Language header shapes, including the common multi-language,
// quality-weighted forms a real browser sends.
func TestNegotiateLanguage(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "empty header has no preference",
			header: "",
			want:   "",
		},
		{
			name:   "single simple language tag",
			header: "fr",
			want:   "fr",
		},
		{
			name:   "language tag is not case sensitive",
			header: "EN",
			want:   "en",
		},
		{
			name:   "region-specific tag falls back to its primary language",
			header: "fr-CA",
			want:   "fr",
		},
		{
			name:   "first listed, highest implied quality, wins",
			header: "fr-CA,fr;q=0.9,en;q=0.8",
			want:   "fr",
		},
		{
			name:   "explicit quality values are honored over list order",
			header: "en;q=0.5,fr;q=0.9",
			want:   "fr",
		},
		{
			name:   "unsupported languages are skipped in favor of a supported one",
			header: "de;q=0.9,fr;q=0.5",
			want:   "fr",
		},
		{
			name:   "no supported language anywhere in the header",
			header: "de,it,xx",
			want:   "",
		},
		{
			name:   "bare wildcard alone matches nothing specific",
			header: "*;q=1.0",
			want:   "",
		},
		{
			name:   "wildcard is ignored when a real supported tag is also present",
			header: "*;q=0.9,es;q=0.1",
			want:   "es",
		},
		{
			name:   "malformed quality value does not break parsing",
			header: "fr;q=not-a-number,en;q=0.9",
			want:   "fr",
		},
		{
			name:   "extra whitespace around tags and parameters is tolerated",
			header: " fr-CA ,  fr ; q=0.9 , en ; q=0.8 ",
			want:   "fr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NegotiateLanguage(tt.header)
			if got != tt.want {
				t.Errorf("NegotiateLanguage(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}
