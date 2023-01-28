package strings

import (
	"bytes"
	"text/template"
	"text/template/parse"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// evaluateTemplate implements the strings.template() function.
func evaluateTemplate(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var err error

	tree, ok := args[0].(*template.Template)
	if !ok {
		return data.List(nil, errors.ErrInvalidType), errors.ErrInvalidType.In("Template()").Context(data.TypeOf(args[0]).String())
	}

	root := tree.Tree.Root

	for _, n := range root.Nodes {
		if n.Type() == parse.NodeTemplate {
			templateNode := n.(*parse.TemplateNode)
			// Get the named template and add it's tree here
			tv, ok := s.Get(templateNode.Name)
			if !ok {
				e := errors.ErrInvalidTemplateName.In("Template()").Context(templateNode.Name)

				return data.List(nil, e), e
			}

			t, ok := tv.(*template.Template)
			if !ok {
				e := errors.ErrInvalidType.In("Template()").Context(data.TypeOf(tv).String())

				return data.List(nil, e), e
			}

			_, err = tree.AddParseTree(templateNode.Name, t.Tree)
			if err != nil {
				return data.List(nil, err), errors.NewError(err)
			}
		}
	}

	var r bytes.Buffer

	if len(args) == 1 {
		err = tree.Execute(&r, nil)
	} else {
		if structure, ok := args[1].(*data.Struct); ok {
			err = tree.Execute(&r, structure.ToMap())
		} else if m, ok := args[1].(*data.Map); ok {
			err = tree.Execute(&r, m.ToMap())
		} else {
			err = tree.Execute(&r, args[1])
		}
	}

	if err != nil {
		err = errors.NewError(err)
	}

	return data.List(r.String(), err), err
}
