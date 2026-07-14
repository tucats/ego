package i18n

import (
	"strings"

	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"

	EgoLocalization "github.com/tucats/ego/internal/i18n"
)

// Implement the i18n.Language() function. Delegates to
// EgoLocalization.DefaultLanguage(), the same process-wide default language
// resolution Ego's own built-in messages use (checks the EGO_LANG
// environment variable, then LANG, then any explicit --language override
// already applied via the CLI), rather than re-deriving it independently.
// An earlier version of this function read LANG directly and truncated it
// at the first "_", which missed the EGO_LANG override entirely and mishandled
// common locale strings with no underscore at all (for example "C.UTF-8", a
// common default in containers, came back unmodified instead of as "C").
func language(s *symbols.SymbolTable, args data.List) (any, error) {
	language := EgoLocalization.DefaultLanguage()
	if language == "" {
		language = "en"
	}

	return language, nil
}

// Implement the i18n.T() function.
//
// Resolution order: an active @localization map for the current language
// (falling back to "en" within that map) takes precedence; if there is no
// @localization map at all, or the key isn't present in it, this falls back
// to Ego's own built-in message catalog (the same one error messages and
// CLI text are drawn from -- see internal/i18n).
//
// Every fallback below calls EgoLocalization.Text(language, property,
// parameters) with the *original* key exactly once, and with the already-
// resolved language (which accounts for the optional third argument -- see
// below). Two bugs existed here previously:
//
//  1. Some fallback paths reassigned property itself to the already-
//     resolved (but not yet substituted) message text before falling
//     through to a second lookup, which meant the resolved text -- not the
//     original key -- was looked up again as if it were itself a catalog
//     key. That second lookup virtually always missed (translated text
//     doesn't coincidentally match another real key) and fell back to
//     returning its input unchanged before substituting, so the net result
//     usually still came out right, but only by accident of that fallback
//     behavior, at the cost of a redundant catalog lookup and a latent risk
//     of picking up an unrelated entry if the resolved text ever did
//     collide with a real key.
//  2. Every fallback called the language-agnostic EgoLocalization.T, which
//     always uses the process-wide default language -- silently discarding
//     an explicit third-argument language override whenever there was no
//     @localization map to satisfy the request instead. Fixed by using
//     EgoLocalization.Text(language, ...), which takes the already-resolved
//     language explicitly.
func translation(s *symbols.SymbolTable, args data.List) (any, error) {
	var (
		property = data.String(args.Get(0))
		language = EgoLocalization.DefaultLanguage()
	)

	// If there is a second function argument, it is a map or struct
	// of the parameters used for the translation. The key value (or
	// field name) is the parameter name, and it's value is the parameter
	// value.
	parameters, err := constructParameterMap(args)
	if err != nil {
		err = errors.New(err)

		return data.NewList(nil, err), err
	}

	// The optional third argument overrides the language derived from
	// the LANG environment variable. If not specified or found, then
	// the default language is "en" for English.
	if args.Len() > 2 {
		language = data.String(args.Get(2))
	}

	if language == "" {
		language = "en"
	}

	language = strings.ToLower(language)

	// Find the localization data value, stored globally by the
	// @localization directive.
	localizedMap, found := s.Get(defs.LocalizationVariable)
	if !found {
		return data.NewList(EgoLocalization.Text(language, property, parameters), nil), nil
	}

	languages, ok := localizedMap.(*data.Struct)
	if !ok {
		return data.NewList(EgoLocalization.Text(language, property, parameters), nil), nil
	}

	// Find the language field in the map, which is the top-level field
	// name, falling back to "en" if the requested language isn't present.
	stringMap, found := languages.Get(language)
	if !found {
		stringMap, found = languages.Get("en")
		if !found {
			return data.NewList(EgoLocalization.Text(language, property, parameters), nil), nil
		}
	}

	localizedStrings, ok := stringMap.(*data.Struct)
	if !ok {
		return data.NewList(EgoLocalization.Text(language, property, parameters), nil), nil
	}

	message, found := localizedStrings.Get(property)
	if !found {
		return data.NewList(EgoLocalization.Text(language, property, parameters), nil), nil
	}

	return format(s, data.NewList(data.String(message), parameters))
}

// If the argument list has more than one argument, the second one will be
// a map or struct used to create the parameter map.
func constructParameterMap(args data.List) (map[string]any, error) {
	parameters := map[string]any{}

	if args.Len() > 1 {
		value := args.Get(1)

		// If we are being called from within format() which was called from
		// translate(), the map is already in the correct format and no work is needed.
		if result, ok := value.(map[string]any); ok {
			return result, nil
		}

		// If this came directly from Ego, it could be an Ego map or struct.
		// Parameter values are kept in their native Go type (int, float64,
		// string, bool, ...) rather than being stringified here: the "%..."
		// format operator (see internal/subs) applies a real Go fmt verb
		// directly to the value, so a numeric verb like "%.2f" needs the
		// actual float64, not a pre-stringified "9.5" -- Go's fmt package
		// can't apply a numeric verb to a string argument (it would print
		// the format-mismatch placeholder "%!f(string=9.5)" instead of a
		// formatted number). The "card"/"zero"/"one"/"many" cardinality
		// operators tolerate either representation (they parse a string
		// back to a number if needed), so this is a strict improvement with
		// no loss of existing behavior.
		if egoMap, ok := value.(*data.Map); ok {
			for _, key := range egoMap.Keys() {
				value, _, _ := egoMap.Get(key)
				parameters[data.String(key)] = value
			}
		} else if egoStruct, ok := value.(*data.Struct); ok {
			for _, field := range egoStruct.FieldNames(false) {
				parameters[field] = egoStruct.GetAlways(field)
			}
		} else if value != nil {
			return nil, errors.ErrArgumentType
		}
	}

	return parameters, nil
}
