package i18n

// Merge accepts a map of localizations and adds it to the existing i18n messages map.
// It returns the count of new localizations added.
func MergeLocalization(additions map[string]map[string]string) int {
	count := mergeLocalizationMap(additions, messages)

	// The merge above may have introduced a language code that wasn't in
	// the catalog before (for example, if an operator loads a brand-new
	// "de" German translation file at runtime via --localization-file).
	// That means any previously cached answer from SupportedLanguages is
	// potentially stale, so discard it -- the next call will recompute it
	// from the now-updated messages map.
	invalidateSupportedLanguagesCache()

	return count
}

// Merge accepts a map of localizations and adds it to an existing map. It returns the
// count of new localizations added.
func mergeLocalizationMap(additions map[string]map[string]string, existing map[string]map[string]string) int {
	count := 0

	for key, value := range additions {
		langs := existing[key]

		// If this is a new message key, assign the new localizations.
		if langs == nil {
			existing[key] = value
			count += len(value)
		} else {
			// Key already existed, so merge in the each language localizations to the existing key.
			for lang, msg := range value {
				langs[lang] = msg
				count++
			}

			// rewrite the updated localizations back into the localizations map.
			existing[key] = langs
		}
	}

	return count
}
