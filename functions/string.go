package functions

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"text/template"
	tparse "text/template/parse"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// Lower implements the lower() function.
func Lower(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	return strings.ToLower(util.GetString(args[0])), nil
}

// Upper implements the upper() function.
func Upper(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	return strings.ToUpper(util.GetString(args[0])), nil
}

// Left implements the left() function.
func Left(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var b strings.Builder

	count := 0
	v := util.GetString(args[0])

	p := datatypes.GetInt(args[1])
	if p <= 0 {
		return "", nil
	}

	for _, ch := range v {
		if count < p {
			b.WriteRune(ch)

			count++
		} else {
			break
		}
	}

	return b.String(), nil
}

// Right implements the right() function.
func Right(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var charPos int

	var b strings.Builder

	v := util.GetString(args[0])

	p := datatypes.GetInt(args[1])
	if p <= 0 {
		return "", nil
	}

	// What's the actual length?
	count := 0
	for range v {
		count++
	}

	for _, ch := range v {
		if charPos >= count-p {
			b.WriteRune(ch)
		}
		charPos++
	}

	return b.String(), nil
}

// Index implements the index() function.
func Index(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	switch arg := args[0].(type) {
	case *datatypes.EgoArray:
		for i := 0; i < arg.Len(); i++ {
			vv, _ := arg.Get(i)
			if reflect.DeepEqual(vv, args[1]) {
				return i, nil
			}
		}

		return -1, nil

	case []interface{}:
		for n, v := range arg {
			if reflect.DeepEqual(v, args[1]) {
				return n, nil
			}
		}

		return -1, nil

	case *datatypes.EgoMap:
		_, found, err := arg.Get(args[1])

		return found, err

	default:
		v := util.GetString(args[0])
		p := util.GetString(args[1])

		return strings.Index(v, p) + 1, nil
	}
}

// Substring implements the substring() function.
func Substring(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	v := util.GetString(args[0])

	p1 := datatypes.GetInt(args[1]) // Starting character position
	if p1 < 1 {
		p1 = 1
	}

	p2 := datatypes.GetInt(args[2]) // Number of characters
	if p2 == 0 {
		return "", nil
	}

	// Calculate length of v in characters
	count := 0
	for range v {
		count++
	}

	// Limit the ending bounds by the actual length
	if p2+p1 > count {
		p2 = count - p1 + 1
	}

	var b strings.Builder

	pos := 1

	for _, ch := range v {
		if pos >= p1+p2 {
			break
		}

		if pos >= p1 {
			b.WriteRune(ch)
		}

		pos++
	}

	return b.String(), nil
}

// Format implements the strings.format() function.
func Format(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) == 0 {
		return "", nil
	}

	if len(args) == 1 {
		return util.GetString(args[0]), nil
	}

	return fmt.Sprintf(util.GetString(args[0]), args[1:]...), nil
}

// Chars implements the strings.chars() function. This accepts a string
// value and converts it to an array of characters.
func Chars(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	count := 0

	// Count the number of characters in the string. (We can't use len() here
	// which onl returns number of bytes)
	v := util.GetString(args[0])
	for i := range v {
		count = i + 1
	}

	r := datatypes.NewArray(datatypes.StringType, count)

	for i, ch := range v {
		err := r.Set(i, string(ch))
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}

// Ints implements the strings.ints() function. This accepts a string
// value and converts it to an array of integer rune values.
func Ints(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	count := 0

	// Count the number of characters in the string. (We can't use len() here
	// which onl returns number of bytes)
	v := util.GetString(args[0])
	for i := range v {
		count = i + 1
	}

	r := datatypes.NewArray(datatypes.IntType, count)

	for i, ch := range v {
		err := r.Set(i, int(ch))
		if err != nil {
			return nil, err
		}
	}

	return r, nil
}

// ToString implements the strings.string() function, which accepts an array
// of items and converts it to a single long string of each item. Normally , this is
// an array of characters.
func ToString(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var b strings.Builder

	for _, v := range args {
		switch a := v.(type) {
		case string:
			b.WriteString(a)

		case int:
			b.WriteRune(rune(a))

		default:
			return nil, errors.New(errors.ErrArgumentCount).In("String()")
		}
	}

	return b.String(), nil
}

// Template implements the strings.template() function.
func Template(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var err error

	if len(args) == 0 {
		return nil, errors.New(errors.ErrArgumentCount).In("Template()")
	}

	tree, ok := args[0].(*template.Template)
	if !ok {
		return nil, errors.New(errors.ErrInvalidType).In("Template()")
	}

	root := tree.Tree.Root

	for _, n := range root.Nodes {
		if n.Type() == tparse.NodeTemplate {
			templateNode := n.(*tparse.TemplateNode)
			// Get the named template and add it's tree here
			tv, ok := s.Get(templateNode.Name)
			if !ok {
				return nil, errors.New(errors.ErrInvalidTemplateName).In("Template()").Context(templateNode.Name)
			}

			t, ok := tv.(*template.Template)
			if !ok {
				return nil, errors.New(errors.ErrInvalidType).In("Template()")
			}

			_, err = tree.AddParseTree(templateNode.Name, t.Tree)
			if !errors.Nil(err) {
				return nil, errors.New(err)
			}
		}
	}

	var r bytes.Buffer

	if len(args) == 1 {
		err = tree.Execute(&r, nil)
	} else {
		if structure, ok := args[1].(*datatypes.EgoStruct); ok {
			err = tree.Execute(&r, structure.ToMap())
		} else {
			err = tree.Execute(&r, args[1])
		}
	}

	return r.String(), errors.New(err)
}

func Truncate(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	name := util.GetString(args[0])
	maxWidth := datatypes.GetInt(args[1])

	if len(name) <= maxWidth {
		return name, nil
	}

	result := name
	chars := 0
	dots := "..."
	limit := maxWidth - len(dots) // name + `...`

	// iterating over strings is based on runes, not bytes.
	for i := range name {
		if chars >= limit {
			result = name[:i] + dots

			break
		}
		chars++
	}

	return result, nil
}

// Split splits a string into lines separated by a newline. Optionally
// a different delimiter can be supplied as the second argument.
func Split(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var v []string

	src := util.GetString(args[0])
	delim := "\n"

	if len(args) > 1 {
		delim = util.GetString(args[1])
	}

	// Are we seeing Windows-style line endings? If we are doing a split
	// based on line endings, use Windows line endings.
	if delim == "\n" && strings.Index(src, "\r\n") > 0 {
		v = strings.Split(src, "\r\n")
	} else {
		// Otherwise, split by the delimiter
		v = strings.Split(src, delim)
	}

	// We need to store the result in a native Ego array.
	r := datatypes.NewArray(datatypes.StringType, len(v))

	for i, n := range v {
		err := r.Set(i, n)
		if err != nil {
			return nil, errors.New(err)
		}
	}

	return r, nil
}

// Tokenize splits a string into tokens.
func Tokenize(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	src := util.GetString(args[0])
	t := tokenizer.New(src)

	r := datatypes.NewArray(datatypes.StringType, len(t.Tokens))

	var err *errors.EgoError

	for i, n := range t.Tokens {
		err = r.Set(i, n)
		if err != nil {
			return nil, err
		}
	}

	return r, err
}

// URLPattern uses ParseURLPattern and then puts the result in a
// native Ego map structure.
func URLPattern(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	result := datatypes.NewMap(datatypes.StringType, datatypes.InterfaceType)

	patternMap, match := ParseURLPattern(util.GetString(args[0]), util.GetString(args[1]))
	if !match {
		return result, nil
	}

	for k, v := range patternMap {
		_, err := result.Set(k, v)
		if err != nil {
			return result, err
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
		// and store in the map using the substitution name
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

// Wrapper around strings.Compare().
func Compare(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	a := util.GetString(args[0])
	b := util.GetString(args[1])

	return strings.Compare(a, b), nil
}

// Wrapper around strings.Contains().
func Contains(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	a := util.GetString(args[0])
	b := util.GetString(args[1])

	return strings.Contains(a, b), nil
}

// Wrapper around strings.Contains().
func ContainsAny(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	a := util.GetString(args[0])
	b := util.GetString(args[1])

	return strings.ContainsAny(a, b), nil
}

// Wrapper around strings.Count().
func Count(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	a := util.GetString(args[0])
	b := util.GetString(args[1])

	return strings.Count(a, b), nil
}

// Wrapper around strings.EqualFold().
func EqualFold(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	a := util.GetString(args[0])
	b := util.GetString(args[1])

	return strings.EqualFold(a, b), nil
}

// Wrapper around strings.Fields().
func Fields(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	a := util.GetString(args[0])

	fields := strings.Fields(a)

	result := datatypes.NewArray(datatypes.StringType, len(fields))

	for idx, f := range fields {
		_ = result.Set(idx, f)
	}

	return result, nil
}

// Wrapper around strings.Join().
func Join(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	elemArray, ok := args[0].(*datatypes.EgoArray)
	if !ok {
		return nil, errors.New(errors.ErrArgumentType).Context("Join()")
	}

	separator := util.GetString(args[1])
	elements := make([]string, elemArray.Len())

	for i := 0; i < elemArray.Len(); i++ {
		element, _ := elemArray.Get(i)
		elements[i] = util.GetString(element)
	}

	return strings.Join(elements, separator), nil
}
