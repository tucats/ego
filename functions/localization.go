package functions

import (
	"bytes"
	"os"
	"strings"
	"text/template"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

func i18nLanguage(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	language := os.Getenv("LANG")

	if pos := strings.Index(language, "_"); pos > 0 {
		language = language[:pos]
	}

	if language == "" {
		language = "en"
	}

	return language, nil
}

func i18nT(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	parameters := map[string]string{}
	property := util.GetString(args[0])

	language := os.Getenv("LANG")

	if pos := strings.Index(language, "_"); pos > 0 {
		language = language[:pos]
	}

	if len(args) > 1 {
		value := args[1]
		if egoMap, ok := value.(*datatypes.EgoMap); ok {
			keys := egoMap.Keys()
			for _, key := range keys {
				value, _, _ := egoMap.Get(key)
				parameters[util.GetString(key)] = util.GetString(value)
			}
		} else if egoStruct, ok := value.(*datatypes.EgoStruct); ok {
			fields := egoStruct.FieldNames()
			for _, field := range fields {
				value := egoStruct.GetAlways(field)
				parameters[field] = util.GetString(value)
			}
		} else if value != nil {
			return nil, errors.New(errors.ErrInvalidArgType)
		}
	}

	if len(args) > 2 {
		language = util.GetString(args[2])
	}

	if language == "" {
		language = "en"
	}

	language = strings.ToLower(language)

	// Find the localization data
	localizedMap, found := s.Get("__localization")
	if !found {
		return property, nil
	}

	// Find the language
	if languages, ok := localizedMap.(*datatypes.EgoStruct); ok {
		stringMap, found := languages.Get(language)
		if !found {
			// If not found, assume english
			stringMap, found = languages.Get("en")
			if !found {
				return property, nil
			}
		}

		if localizedStrings, ok := stringMap.(*datatypes.EgoStruct); ok {
			message, found := localizedStrings.Get(property)
			if !found {
				return property, nil
			}

			msgString := util.GetString(message)
			t := template.New(property)

			t, e := t.Parse(msgString)
			if e != nil {
				return nil, errors.New(e)
			}

			var r bytes.Buffer

			e = t.Execute(&r, parameters)
			if e != nil {
				return nil, errors.New(e)
			}

			return r.String(), nil
		}
	}

	return property, nil
}
