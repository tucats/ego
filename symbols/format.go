package symbols

import (
	"sort"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// Format formats a symbol table into a string for printing/display.
func (s *SymbolTable) Format(includeBuiltins bool) string {
	var b strings.Builder

	b.WriteString("Symbol table")

	if s.Name != "" {
		b.WriteString(" \"")
		b.WriteString(s.Name)
		b.WriteString("\"")
	}

	b.WriteString(":\n")

	// Iterate over the members to get a list of the keys. Discard invisible
	// items.
	keys := make([]string, 0)

	for k := range s.Symbols {
		if !strings.HasPrefix(k, "__") {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

	// Now iterate over the keys in sorted order
	for _, k := range keys {
		// reserved words are not valid symbol names
		if tokenizer.IsReserved(k, false) {
			continue
		}

		v := s.Symbols[k]
		skip := false
		typeString := "package"

		switch actual := v.(type) {
		case *datatypes.EgoMap:
			typeString = actual.TypeString()

		case func(*SymbolTable, []interface{}) (interface{}, error):
			if !includeBuiltins {
				continue
			}

		case map[string]interface{}:
			if tsx, ok := datatypes.GetMetadata(actual, datatypes.TypeMDKey); ok {
				typeString = util.GetString(tsx)
			}

			for _, k2 := range actual {
				if _, ok := k2.(func(*SymbolTable, []interface{}) (interface{}, error)); ok {
					skip = true
				}
			}

			if skip && !includeBuiltins {
				continue
			}
		}

		b.WriteString("   ")
		b.WriteString(k)
		b.WriteString(" = ")

		if skip {
			b.WriteString("(")
			b.WriteString(typeString)
			b.WriteString(") ")
		}

		// Any variable named _password or _token has it's value obscured
		if k == "_password" || k == "_token" {
			b.WriteString("\"******\"")
		} else {
			b.WriteString(util.Format(v))
		}

		b.WriteString("\n")
	}

	if s.Parent != nil {
		sp := s.Parent.Format(includeBuiltins)

		b.WriteString("\n")
		b.WriteString(sp)
	}

	return b.String()
}
