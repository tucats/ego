package symbols

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
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

	b.WriteString(fmt.Sprintf(" (%d/%d):\n",
		s.ValueSize, len(s.Values)))

	// Iterate over the members to get a list of the keys. Discard invisible
	// items.
	keys := make([]string, 0)

	for k := range s.Symbols {
		if !strings.HasPrefix(k, datatypes.MetadataPrefix) && !strings.HasPrefix(k, "$") {
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

		v := s.GetValue(s.Symbols[k])
		omitType := false
		omitThisSymbol := false

		dt := datatypes.TypeOf(v)
		typeString := dt.String()

		switch actual := v.(type) {
		case *datatypes.EgoMap:
			typeString = actual.TypeString()

		case *datatypes.EgoArray:
			typeString = actual.TypeString()

		case *datatypes.EgoStruct:
			typeString = actual.TypeString()

		case datatypes.EgoPackage:
			if tsx, ok := datatypes.GetMetadata(actual, datatypes.TypeMDKey); ok {
				typeString = datatypes.GetString(tsx)
			}

			hasBuiltins := false
			keys := actual.Keys()

			for _, k := range keys {
				k2, _ := actual.Get(k)
				if _, ok := k2.(func(*SymbolTable, []interface{}) (interface{}, *errors.EgoError)); ok {
					hasBuiltins = true
					omitType = true
				}
			}

			if hasBuiltins && !includeBuiltins {
				omitThisSymbol = true

				continue
			}

		case func(*SymbolTable, []interface{}) (interface{}, *errors.EgoError):
			if !includeBuiltins {
				omitThisSymbol = true
			}

			typeString = "builtin"

		default:
			reflectedData := fmt.Sprintf("%#v", actual)
			if strings.HasPrefix(reflectedData, "&bytecode.ByteCode") {
				typeString = "func"

				if !includeBuiltins {
					omitType = true
				}
			}
		}

		if omitThisSymbol {
			continue
		}

		b.WriteString("   ")
		b.WriteString(k)

		if !omitType {
			b.WriteString(" <")
			b.WriteString(typeString)
			b.WriteString(">")
		}

		b.WriteString(" = ")

		// Any variable named _password or _token has it's value obscured
		if k == "_password" || k == "_token" {
			b.WriteString("\"******\"")
		} else {
			b.WriteString(datatypes.Format(v))
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

// Format formats a symbol table into a string for printing/display.
func (s *SymbolTable) FormattedData(includeBuiltins bool) [][]string {
	rows := make([][]string, 0)

	// Iterate over the members to get a list of the keys. Discard invisible
	// items.
	keys := make([]string, 0)

	for k := range s.Symbols {
		if !strings.HasPrefix(k, datatypes.MetadataPrefix) && !strings.HasPrefix(k, "$") {
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

		v := s.GetValue(s.Symbols[k])
		omitThisSymbol := false

		dt := datatypes.TypeOf(v)
		typeString := dt.String()

		switch actual := v.(type) {
		case *datatypes.EgoMap:
			typeString = actual.TypeString()

		case *datatypes.EgoArray:
			typeString = actual.TypeString()

		case *datatypes.EgoStruct:
			typeString = actual.TypeString()

		case datatypes.EgoPackage:
			if tsx, ok := datatypes.GetMetadata(actual, datatypes.TypeMDKey); ok {
				typeString = datatypes.GetString(tsx)
			}

			hasBuiltins := false
			keys := actual.Keys()

			for _, k := range keys {
				k2, _ := actual.Get(k)
				if _, ok := k2.(func(*SymbolTable, []interface{}) (interface{}, *errors.EgoError)); ok {
					hasBuiltins = true
				}
			}

			if hasBuiltins && !includeBuiltins {
				omitThisSymbol = true

				continue
			}

		case func(*SymbolTable, []interface{}) (interface{}, *errors.EgoError):
			if !includeBuiltins {
				omitThisSymbol = true
			}

			typeString = "builtin"

		default:
			reflectedData := fmt.Sprintf("%#v", actual)
			if strings.HasPrefix(reflectedData, "&bytecode.ByteCode") {
				typeString = "func"
			}
		}

		if omitThisSymbol {
			continue
		}

		row := make([]string, 3)
		row[0] = k
		row[1] = typeString

		// Any variable named _password or _token has it's value obscured
		if k == "_password" || k == "_token" {
			v = "\"******\""
		}

		row[2] = datatypes.Format(v)
		rows = append(rows, row)
	}

	return rows
}
