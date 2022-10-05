package datatypes

import "strings"

type FunctionParameter struct {
	Name     string
	ParmType Type
}

type FunctionDeclaration struct {
	Name        string
	Parameters  []FunctionParameter
	ReturnTypes []Type
}

func (f FunctionDeclaration) String() string {
	r := strings.Builder{}
	r.WriteString(f.Name)
	r.WriteRune('(')

	for i, p := range f.Parameters {
		if i > 0 {
			r.WriteString(", ")
		}

		r.WriteString(p.Name)
		r.WriteRune(' ')
		r.WriteString(p.ParmType.String())
	}

	r.WriteString(")")

	if len(f.ReturnTypes) > 0 {
		if len(f.ReturnTypes) > 1 {
			r.WriteRune('(')
		}

		for i, p := range f.ReturnTypes {
			if i > 0 {
				r.WriteRune(',')
			}

			r.WriteString(p.String())
		}

		if len(f.ReturnTypes) > 1 {
			r.WriteRune(')')
		}
	}

	return r.String()
}
