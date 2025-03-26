package i18n

import (
	"bytes"
	"os"
	"strings"
	"text/template"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

func language(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	language := os.Getenv("LANG")

	if pos := strings.Index(language, "_"); pos > 0 {
		language = language[:pos]
	}

	if language == "" {
		language = "en"
	}

	return language, nil
}

func translation(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	var (
		r        bytes.Buffer
		property = data.String(args.Get(0))
		language = os.Getenv("LANG")
	)

	if pos := strings.Index(language, "_"); pos > 0 {
		language = language[:pos]
	}

	// If there is a second function argument, it is a map or struct
	// of the parameters used for the translation. The key value (or
	// field name) is the parameter name, and it's value is the parameter
	// value.
	parameters, err := constructParameterMap(args)
	if err != nil {
		return nil, err
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

	// Find the localization data value, stored globallly.
	localizedMap, found := s.Get(defs.LocalizationVariable)
	if !found {
		return property, nil
	}

	// Find the language field in the map, which is the top-level field
	// name.
	if languages, ok := localizedMap.(*data.Struct); ok {
		stringMap, found := languages.Get(language)
		if !found {
			// If not found, assume english
			stringMap, found = languages.Get("en")
			if !found {
				return property, nil
			}
		}

		if localizedStrings, ok := stringMap.(*data.Struct); ok {
			message, found := localizedStrings.Get(property)
			if !found {
				return property, nil
			}

			msgString := data.String(message)
			t := template.New(property)

			t, e := t.Parse(msgString)
			if e != nil {
				return nil, errors.New(e)
			}

			e = t.Execute(&r, parameters)
			if e != nil {
				return nil, errors.New(e)
			}

			return r.String(), nil
		}
	}

	return property, nil
}

// If the argument list has more than one argument, the second one will be
// a map or struct used to create the paraemter map.
func constructParameterMap(args data.List) (interface{}, error) {
	parameters := map[string]string{}

	if args.Len() > 1 {
		value := args.Get(1)
		if egoMap, ok := value.(*data.Map); ok {
			for _, key := range egoMap.Keys() {
				value, _, _ := egoMap.Get(key)
				parameters[data.String(key)] = data.String(value)
			}
		} else if egoStruct, ok := value.(*data.Struct); ok {
			for _, field := range egoStruct.FieldNames(false) {
				value := egoStruct.GetAlways(field)
				parameters[field] = data.String(value)
			}
		} else if value != nil {
			return nil, errors.ErrArgumentType
		}
	}

	return parameters, nil
}
