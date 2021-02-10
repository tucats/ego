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

	p := util.GetInt(args[1])
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
	var cpos int

	var b strings.Builder

	v := util.GetString(args[0])

	p := util.GetInt(args[1])
	if p <= 0 {
		return "", nil
	}

	// What's the actual length?
	count := 0
	for range v {
		count++
	}

	for _, ch := range v {
		if cpos >= count-p {
			b.WriteRune(ch)
		}
		cpos++
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

	case datatypes.EgoMap:
		_, found, err := arg.Get(args[1])

		return found, err

	case map[string]interface{}:
		key := util.GetString(args[1])
		_, found := arg[key]

		return found, nil

	default:
		v := util.GetString(args[0])
		p := util.GetString(args[1])

		return strings.Index(v, p) + 1, nil
	}
}

// Substring implements the substring() function.
func Substring(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	v := util.GetString(args[0])

	p1 := util.GetInt(args[1]) // Starting character position
	if p1 < 1 {
		p1 = 1
	}

	p2 := util.GetInt(args[2]) // Number of characters
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
	v := util.GetString(args[0])
	r := make([]interface{}, 0)

	for _, ch := range v {
		r = append(r, ch)
	}

	return r, nil
}

// Ints implements the strings.ints() function. This accepts a string
// value and converts it to an array of integer rune values.
func Ints(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	v := util.GetString(args[0])
	r := make([]interface{}, 0)
	i := []rune(v)

	for n := 0; n < len(i); n = n + 1 {
		r = append(r, int(i[n]))
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

		case []interface{}:
			for _, c := range a {
				switch k := c.(type) {
				case int:
					b.WriteRune(rune(k))

				case string:
					b.WriteString(util.GetString(c))

				default:
					return nil, errors.New(errors.InvalidTypeError).In("string()")
				}
			}

		default:
			return nil, errors.New(errors.ArgumentCountError).In("string()")
		}
	}

	return b.String(), nil
}

// Template implements the strings.template() function.
func Template(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var err error

	if len(args) == 0 {
		return nil, errors.New(errors.ArgumentCountError).In("template()")
	}

	tree, ok := args[0].(*template.Template)
	if !ok {
		return nil, errors.New(errors.InvalidTypeError).In("template()")
	}

	root := tree.Tree.Root

	for _, n := range root.Nodes {
		//fmt.Printf("Node[%2d]: %#v\n", i, n)
		if n.Type() == tparse.NodeTemplate {
			templateNode := n.(*tparse.TemplateNode)
			// Get the named template and add it's tree here
			tv, ok := s.Get(templateNode.Name)
			if !ok {
				return nil, errors.New(errors.InvalidTemplateName).In("template()").WithContext(templateNode.Name)
			}

			t, ok := tv.(*template.Template)
			if !ok {
				return nil, errors.New(errors.InvalidTypeError).In("template()")
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
		err = tree.Execute(&r, args[1])
	}

	return r.String(), errors.New(err)
}

func Truncate(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	name := util.GetString(args[0])
	maxWidth := util.GetInt(args[1])

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
