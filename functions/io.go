package functions

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// ReadFile reads a file contents into a string value.
func ReadFile(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	name := util.GetString(args[0])
	if name == "." {
		return ui.Prompt(""), nil
	}

	content, err := ioutil.ReadFile(name)
	if !errors.Nil(err) {
		return nil, errors.New(err)
	}

	// Convert []byte to string
	return string(content), nil
}

// Split splits a string into lines.
func Split(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	src := util.GetString(args[0])

	// Are we seeing Windows-style line endings? If so, use that as
	// the split boundary.
	if strings.Index(src, "\r\n") > 0 {
		return strings.Split(src, "\r\n"), nil
	}

	// Otherwise, simple split by new-line works fine.
	v := strings.Split(src, "\n")
	r := make([]interface{}, 0)

	// We must recopy this into an array of interfaces to adopt Ego typelessness.
	for _, n := range v {
		r = append(r, n)
	}

	return r, nil
}

// Tokenize splits a string into tokens.
func Tokenize(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	src := util.GetString(args[0])
	t := tokenizer.New(src)
	r := make([]interface{}, 0)

	// We must recopy this into an array of interfaces to adopt Ego typelessness.
	for _, n := range t.Tokens {
		r = append(r, n)
	}

	return r, nil
}

// WriteFile writes a string to a file.
func WriteFile(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	fname := util.GetString(args[0])
	text := util.GetString(args[1])
	err := ioutil.WriteFile(fname, []byte(text), 0777)

	return len(text), errors.New(err)
}

// DeleteFile delete a file.
func DeleteFile(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	fname := util.GetString(args[0])
	err := os.Remove(fname)

	return errors.Nil(err), errors.New(err)
}

// Expand expands a list of file or path names into a list of files.
func Expand(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	path := util.GetString(args[0])
	ext := ""

	if len(args) > 1 {
		ext = util.GetString(args[1])
	}

	list, err := ExpandPath(path, ext)

	// Rewrap as an interface array
	result := []interface{}{}

	for _, item := range list {
		result = append(result, item)
	}

	return result, err
}

// ExpandPath is used to expand a path into a list of fie names.
func ExpandPath(path, ext string) ([]string, *errors.EgoError) {
	names := []string{}

	// Can we read this as a directory?
	fi, err := ioutil.ReadDir(path)
	if !errors.Nil(err) {
		fn := path

		_, err := ioutil.ReadFile(fn)
		if !errors.Nil(err) {
			fn = path + ext
			_, err = ioutil.ReadFile(fn)
		}

		if !errors.Nil(err) {
			return names, errors.New(err)
		}

		// If we have a default suffix, make sure the pattern matches
		if ext != "" && !strings.HasSuffix(fn, ext) {
			return names, nil
		}

		names = append(names, fn)

		return names, nil
	}

	// Read as a directory
	for _, f := range fi {
		fn := filepath.Join(path, f.Name())

		list, err := ExpandPath(fn, ext)
		if !errors.Nil(err) {
			return names, err
		}

		names = append(names, list...)
	}

	return names, nil
}

// ReadDir implements the io.readdir() function.
func ReadDir(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	path := util.GetString(args[0])
	result := []interface{}{}

	files, err := ioutil.ReadDir(path)
	if !errors.Nil(err) {
		return result, errors.New(err).In("ReadDir()")
	}

	for _, file := range files {
		entry := map[string]interface{}{}
		entry["name"] = file.Name()
		entry["directory"] = file.IsDir()
		entry["mode"] = file.Mode().String()
		entry["size"] = int(file.Size())
		entry["modified"] = file.ModTime().String()
		result = append(result, entry)
	}

	return result, nil
}

// This is the generic close() which can be used to close a channel, and maybe
// later other items as well.
func CloseAny(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	switch arg := args[0].(type) {
	case *datatypes.Channel:
		return arg.Close(), nil

	default:
		return nil, errors.New(errors.InvalidTypeError).In("CloseAny()")
	}
}

// URLPattern uses ParseURLPattern and then puts the result in a
// native Ego map structure.
func URLPattern(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 2 {
		return nil, errors.New(errors.ArgumentCountError).In("urlpattern()")
	}

	result := datatypes.NewMap(datatypes.StringType, datatypes.InterfaceType)

	patternMap, match := ParseURLPattern(util.GetString(args[0]), util.GetString(args[1]))
	if !match {
		return result, nil
	}

	for k, v := range patternMap {
		_, err := result.Set(k, v)
		if err != nil {
			return result, errors.New(err)
		}
	}

	return result, nil
}

// ParseURLPattern accepts a pattern that tells what part of the URL is
// meant to be literal, and what is a user-supplied item. The result is
// a map of the URL items parsed.
//
// If the pattern is
//
//   "/services/debug/processes/{{ID}}"
//
// and the url is
//
//   /services/debug/processses/1653
//
// Then the result map will be
//    map[string]interface{} {
//             "ID" : 1653
//    }
func ParseURLPattern(url, pattern string) (map[string]interface{}, bool) {
	urlParts := strings.Split(strings.ToLower(url), "/")
	patternParts := strings.Split(strings.ToLower(pattern), "/")
	result := map[string]interface{}{}

	if len(urlParts) > len(patternParts) {
		return nil, false
	}

	for idx, pat := range patternParts {
		if len(pat) == 0 {
			continue
		}

		// If the pattern continues longer than the
		// URL given, mark those as being absent
		if idx >= len(urlParts) {
			// Is this part of the pattern a substitution? If not, we store
			// it in the result as a field-not-found. If it is a substitution
			// operator, store as an empty string.
			if !strings.HasPrefix(pat, "{{") || !strings.HasSuffix(pat, "}}") {
				result[pat] = false
			} else {
				name := strings.Replace(strings.Replace(pat, "{{", "", 1), "}}", "", 1)
				result[name] = ""
			}

			continue
		}

		// If this part just matches, mark it as present.
		if pat == urlParts[idx] {
			result[pat] = true

			continue
		}

		// If this pattern is a substitution operator, get the value now
		// and store in the maap using the substitution name
		if strings.HasPrefix(pat, "{{") && strings.HasSuffix(pat, "}}") {
			name := strings.Replace(strings.Replace(pat, "{{", "", 1), "}}", "", 1)
			result[name] = urlParts[idx]
		} else {
			// It didn't match the url, so no data
			return nil, false
		}
	}

	return result, true
}
