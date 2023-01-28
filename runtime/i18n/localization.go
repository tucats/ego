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

func Language(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount
	}

	language := os.Getenv("LANG")

	if pos := strings.Index(language, "_"); pos > 0 {
		language = language[:pos]
	}

	if language == "" {
		language = "en"
	}

	return language, nil
}

func T(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	parameters := map[string]string{}
	property := data.String(args[0])

	language := os.Getenv("LANG")

	if pos := strings.Index(language, "_"); pos > 0 {
		language = language[:pos]
	}

	if len(args) > 1 {
		value := args[1]
		if egoMap, ok := value.(*data.Map); ok {
			for _, key := range egoMap.Keys() {
				value, _, _ := egoMap.Get(key)
				parameters[data.String(key)] = data.String(value)
			}
		} else if egoStruct, ok := value.(*data.Struct); ok {
			for _, field := range egoStruct.FieldNames() {
				value := egoStruct.GetAlways(field)
				parameters[field] = data.String(value)
			}
		} else if value != nil {
			return nil, errors.ErrArgumentType
		}
	}

	if len(args) > 2 {
		language = data.String(args[2])
	}

	if language == "" {
		language = "en"
	}

	language = strings.ToLower(language)

	// Find the localization data
	localizedMap, found := s.Get(defs.LocalizationVariable)
	if !found {
		return property, nil
	}

	// Find the language
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
				return nil, errors.NewError(e)
			}

			var r bytes.Buffer

			e = t.Execute(&r, parameters)
			if e != nil {
				return nil, errors.NewError(e)
			}

			return r.String(), nil
		}
	}

	return property, nil
}
