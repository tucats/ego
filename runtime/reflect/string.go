package reflect

import (
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func getString(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	r := getThis(s)
	t := strings.Builder{}

	t.WriteString("Reflection{")

	comma := false

	if v := r.GetAlways(data.NameMDName); v != nil {
		t.WriteString("Name: ")
		t.WriteString(data.String(v))

		comma = true
	}

	if v := r.GetAlways(data.TypeMDName); v != nil {
		if comma {
			t.WriteString(", ")
		}

		t.WriteString("Type: ")
		t.WriteString(data.String(v))
	}

	if v := r.GetAlways(data.IsTypeMDName); v != nil {
		t.WriteString(", ")
		t.WriteString("IsType: ")
		t.WriteString(data.String(v))
	}

	if v := r.GetAlways(data.BasetypeMDName); v != nil {
		t.WriteString(", ")
		t.WriteString("Basetype: ")
		t.WriteString(data.String(v))
	}

	if v := r.GetAlways(data.BuiltinsMDName); v != nil {
		t.WriteString(", ")
		t.WriteString("Builtins: ")
		t.WriteString(data.String(v))
	}

	if v := r.GetAlways(data.ImportsMDName); v != nil {
		t.WriteString(", ")
		t.WriteString("Imports: ")
		t.WriteString(data.String(v))
	}

	if v := r.GetAlways(data.TextMDName); v != nil {
		t.WriteString(", ")
		t.WriteString("Error: ")
		t.WriteString(data.String(v))
	}

	if v := r.GetAlways(data.ContextMDName); v != nil {
		t.WriteString(", ")
		t.WriteString("Context: ")
		t.WriteString(data.String(v))
	}

	if v := r.GetAlways(data.DeclarationMDName); v != nil {
		t.WriteString(", ")
		t.WriteString("Declaration: ")
		t.WriteString(data.String(v))
	}

	if v := r.GetAlways(data.MembersMDName); v != nil {
		t.WriteString(", ")
		t.WriteString("Members: ")
		t.WriteString(data.String(v))
	}

	if v := r.GetAlways(data.FunctionsMDName); v != nil {
		if _, ok := v.(data.Function); !ok {
			t.WriteString(", ")
			t.WriteString("Functions: ")
			t.WriteString(data.String(v))
		}
	}

	if v := r.GetAlways(data.SizeMDName); v != nil {
		t.WriteString(", ")
		t.WriteString("Size: ")
		t.WriteString(data.String(v))
	}

	t.WriteString("}")

	return t.String(), nil
}
