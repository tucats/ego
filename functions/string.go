package functions

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"text/template"
	tparse "text/template/parse"

	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Lower implements the lower() function
func Lower(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return strings.ToLower(util.GetString(args[0])), nil
}

// Upper implements the upper() function
func Upper(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	return strings.ToUpper(util.GetString(args[0])), nil
}

// Left implements the left() function
func Left(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	v := util.GetString(args[0])
	p := util.GetInt(args[1])

	if p <= 0 {
		return "", nil
	}
	if p >= len(v) {
		return v, nil
	}
	return v[:p], nil
}

// Right implements the right() function
func Right(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	v := util.GetString(args[0])
	p := util.GetInt(args[1])

	if p <= 0 {
		return "", nil
	}
	if p >= len(v) {
		return v, nil
	}
	return v[len(v)-p:], nil
}

// Index implements the index() function
func Index(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	switch arg := args[0].(type) {

	case []interface{}:
		for n, v := range arg {
			if reflect.DeepEqual(v, args[1]) {
				return n + 1, nil
			}
		}
		return 0, nil

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

// Substring implements the substring() function
func Substring(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	v := util.GetString(args[0])
	p1 := util.GetInt(args[1])
	p2 := util.GetInt(args[2])

	if p1 < 1 {
		p1 = 1
	}
	if p2 == 0 {
		return "", nil
	}
	if p2+p1 > len(v) {
		p2 = len(v) - p1 + 1
	}

	s := v[p1-1 : p1+p2-1]
	return s, nil
}

// Format implements the strings.format() function
func Format(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {

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
func Chars(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	v := util.GetString(args[0])
	r := make([]interface{}, 0)

	for n := 0; n < len(v); n = n + 1 {
		r = append(r, v[n:n+1])
	}
	return r, nil
}

// Ints implements the strings.ints() function. This accepts a string
// value and converts it to an array of integer rune values.
func Ints(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {

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
func ToString(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {

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
					return nil, NewError("string", InvalidTypeError)
				}
			}
		default:
			return nil, NewError("string", ArgumentCountError)
		}
	}
	return b.String(), nil

}

// Template implements the strings.template() function
func Template(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var err error
	if len(args) == 0 {
		return nil, NewError("template", ArgumentCountError)
	}
	tree, ok := args[0].(*template.Template)
	if !ok {
		return nil, NewError("string", InvalidTypeError)
	}

	root := tree.Tree.Root
	for _, n := range root.Nodes {
		//fmt.Printf("Node[%2d]: %#v\n", i, n)
		if n.Type() == tparse.NodeTemplate {
			templateNode := n.(*tparse.TemplateNode)
			// Get the named template and add it's tree here
			tv, ok := s.Get(templateNode.Name)
			if !ok {
				return nil, NewError("template", InvalidTemplateNameError, templateNode.Name)
			}
			t, ok := tv.(*template.Template)
			if !ok {
				return nil, NewError("template", InvalidTypeError, templateNode.Name)
			}
			_, err = tree.AddParseTree(templateNode.Name, t.Tree)
			if err != nil {
				return nil, err
			}
		}
	}

	var r bytes.Buffer
	if len(args) == 1 {
		err = tree.Execute(&r, nil)
	} else {
		err = tree.Execute(&r, args[1])
	}
	return r.String(), err
}

func Truncate(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {

	name := util.GetString(args[0])
	maxWidth := util.GetInt(args[1])
	if len(name) <= maxWidth {
		return name, nil
	}
	result := name
	chars := 0
	limit := maxWidth - 3 // name + `...`
	// iterating over strings is based on runes, not bytes.
	for i := range name {
		if chars >= limit {
			result = name[:i] + `...`
			break
		}
		chars++
	}
	return result, nil
}
