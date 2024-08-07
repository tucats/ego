package symbols

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/tokenizer"
)

const (
	builtinTypeName = "builtin"
	funcTypeName    = "func"
)

var formatMutex sync.Mutex

// Format formats a symbol table into a string for printing/display.
func (s *SymbolTable) Format(includeBuiltins bool) string {
	formatMutex.Lock()
	defer formatMutex.Unlock()

	if s == nil {
		return "<symbol table is nil>\n"
	}

	return s.formatWithLevel(0, includeBuiltins)
}
func (s *SymbolTable) formatWithLevel(level int, includeBuiltins bool) string {
	var b strings.Builder

	b.WriteString("Symbol table")

	if s.Name != "" {
		b.WriteString(" \"")
		b.WriteString(s.Name)
		b.WriteString("\"")
	}

	flags := fmt.Sprintf(" <level %d, id %s, ", level, s.id.String())

	if s.modified {
		flags += "modified, "
	}

	if s.shared {
		flags += "shared, "
	}

	if s.isClone {
		flags += "clone, "
	}

	if s.isRoot {
		flags += "root, "
	}

	if s.boundary {
		flags += "boundary, "
	}

	if s.forPackage != "" {
		flags += fmt.Sprintf("package %s, ", s.forPackage)
	}

	flags += fmt.Sprintf("len=%d, bins=%d>\n", s.size, len(s.values))

	b.WriteString(flags)

	// Show the raw pointer
	b.WriteString(fmt.Sprintf("   Raw pointer %p\n", s))

	// Show the parent
	if parent := s.Parent(); parent != nil {
		b.WriteString(fmt.Sprintf("   Parent table %s (%s)\n",
			parent.Name, parent.ID().String()))
	}

	// Iterate over the members to get a list of the keys. Discard invisible
	// items.
	keys := make([]string, 0)

	for k := range s.symbols {
		if !strings.HasPrefix(k, data.MetadataPrefix) && !strings.HasPrefix(k, "$") {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

	// Now iterate over the keys in sorted order
	for _, k := range keys {
		// reserved words are not valid symbol names
		if tokenizer.NewReservedToken(k).IsReserved(false) {
			continue
		}

		v := s.GetValue(s.symbols[k].slot)
		omitType := false
		omitThisSymbol := false

		dt := data.TypeOf(v)
		typeString := dt.String()

		switch actual := v.(type) {
		case *data.Map:
			typeString = actual.TypeString()

		case *data.Array:
			typeString = actual.TypeString()

		case *data.Struct:
			typeString = actual.TypeString()

		case *data.Package:
			if tsx, ok := actual.Get(data.TypeMDKey); ok {
				typeString = data.String(tsx)
			} else {
				typeString = "package"
			}

		case func(*SymbolTable, []interface{}) (interface{}, error):
			if !includeBuiltins {
				omitThisSymbol = true
			}

			typeString = builtinTypeName

		case *data.Type:
			typeString = "type"

		default:
			reflectedData := fmt.Sprintf("%#v", actual)
			if strings.HasPrefix(reflectedData, "&bytecode.ByteCode") {
				typeString = funcTypeName

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
		if k == defs.PasswordVariable || k == defs.TokenVariable {
			b.WriteString("\"******\"")
		} else {
			b.WriteString(data.Format(v))
		}

		b.WriteString("\n")
	}

	if s.parent != nil {
		sp := s.parent.formatWithLevel(level+1, includeBuiltins)

		b.WriteString("\n")
		b.WriteString(sp)
	}

	return b.String()
}

// Format formats a symbol table into a string for printing/display.
func (s *SymbolTable) Log(session int, logger int) {
	if !ui.IsActive(logger) {
		return
	}

	name := s.Name
	if name != "" {
		name = " " + strconv.Quote(name)
	}

	ui.Log(logger, "[%d] Symbol table%s, id=%s, count=%d. segments=%d", session, name, s.id.String(), s.size, len(s.values))

	// Iterate over the members to get a list of the keys. Discard invisible
	// items.
	keys := make([]string, 0)

	for k := range s.symbols {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	// Now iterate over the keys in sorted order
	for _, k := range keys {
		v := s.GetValue(s.symbols[k].slot)

		typeString := ""

		switch actual := v.(type) {
		case bool, byte, int, int32, int64, string, float32, float64:
			typeString = data.TypeOf(v).String()

		case *data.Type:
			typeString = "type"

		case *data.Map:
			typeString = actual.TypeString()

		case *data.Array:
			typeString = actual.TypeString()

		case *data.Struct:
			typeString = actual.TypeString()

		case *data.Package:
			if tsx, ok := actual.Get(data.TypeMDKey); ok {
				typeString = data.String(tsx)
			} else {
				typeString = "package"
			}

		case func(*SymbolTable, []interface{}) (interface{}, error):
			typeString = builtinTypeName

		case func(*SymbolTable, data.List) (interface{}, error):
			typeString = builtinTypeName

		default:
			reflectedData := fmt.Sprintf("%#v", actual)
			if strings.HasPrefix(reflectedData, "&bytecode.ByteCode") {
				typeString = funcTypeName
			}
		}

		value := strconv.Quote("********")
		if k != defs.PasswordVariable && k != defs.TokenVariable {
			value = data.Format(v)
		}

		ui.Log(logger, "[%d]   %-16s %s = %s", session, k, typeString, value)
	}

	if s.parent != nil {
		s.parent.Log(session, logger)
	}
}

// Format formats a symbol table into a string for printing/display.
func (s *SymbolTable) FormattedData(includeBuiltins bool) [][]string {
	rows := make([][]string, 0)

	// Iterate over the members to get a list of the keys. Discard invisible
	// items.
	keys := make([]string, 0)

	for k := range s.symbols {
		if !strings.HasPrefix(k, data.MetadataPrefix) && !strings.HasPrefix(k, "$") {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

	// Now iterate over the keys in sorted order
	for _, k := range keys {
		// reserved words are not valid symbol names
		if tokenizer.NewReservedToken(k).IsReserved(false) {
			continue
		}

		attr := s.symbols[k]
		v := s.GetValue(attr.slot)
		omitThisSymbol := false

		dt := data.TypeOf(v)
		typeString := dt.String()

		switch actual := v.(type) {
		case *data.Map:
			typeString = actual.TypeString()

		case *data.Array:
			typeString = actual.TypeString()

		case *data.Struct:
			typeString = actual.TypeString()

		case *data.Package:
			if tsx, ok := actual.Get(data.TypeMDKey); ok {
				typeString = data.String(tsx)
			}

			hasBuiltins := false
			keys := actual.Keys()

			for _, k := range keys {
				k2, _ := actual.Get(k)
				if _, ok := k2.(func(*SymbolTable, []interface{}) (interface{}, error)); ok {
					hasBuiltins = true
				}
			}

			if hasBuiltins && !includeBuiltins {
				omitThisSymbol = true

				continue
			}

		case func(*SymbolTable, []interface{}) (interface{}, error):
			if !includeBuiltins {
				omitThisSymbol = true
			}

			typeString = builtinTypeName

		default:
			reflectedData := fmt.Sprintf("%#v", actual)
			if strings.HasPrefix(reflectedData, "&bytecode.ByteCode") {
				typeString = funcTypeName
			}
		}

		if omitThisSymbol {
			continue
		}

		row := make([]string, 4)
		row[0] = k
		row[1] = typeString

		// Any variable named _password or _token has it's value obscured
		if k == defs.PasswordVariable || k == defs.TokenVariable {
			v = "\"******\""
		}

		row[2] = data.String(attr.Readonly)
		row[3] = data.Format(v)
		rows = append(rows, row)
	}

	return rows
}
