package symbols

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/tokenizer"
)

const (
	builtinTypeName = "builtin"
	funcTypeName    = "func"
)

var formatMutex sync.Mutex

func (s *SymbolTable) String() string {
	text := fmt.Sprintf("Symbol table %s %s", s.Name, tableFlagsString(s, -1))

	return text
}

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

	flags := tableFlagsString(s, level)

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
	keys := getVisibleSymbolNames(s)

	// Now iterate over the keys in sorted order
	for _, k := range keys {
		// reserved words are not valid symbol names
		if tokenizer.NewReservedToken(k).IsReserved(false) {
			continue
		}

		v := s.getValue(s.symbols[k].slot)
		omitType, omitSymbol, typeString := getFormattedTypeString(v, includeBuiltins)

		if omitSymbol {
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
		b.WriteString(data.Format(v))

		b.WriteString("\n")
	}

	if s.parent != nil {
		sp := s.parent.formatWithLevel(level+1, includeBuiltins)

		b.WriteString("\n")
		b.WriteString(sp)
	}

	return b.String()
}

// getFormattedTypeString returns the type string for a given value. It also determines
// if the resulting type indicates that it should be omitted from the output.
func getFormattedTypeString(v any, includeBuiltins bool) (bool, bool, string) {
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

	case func(*SymbolTable, data.List) (any, error):
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

	return omitType, omitThisSymbol, typeString
}

// For the given symbol table, return a sorted list of the visible symbol names
// in the table, suitable for formatted output. This specifically omits any variable
// with the hidden "__" prefix or a generated symbol that arts with a "$" prefix.
func getVisibleSymbolNames(s *SymbolTable) []string {
	keys := make([]string, 0)

	for k := range s.symbols {
		if !strings.HasPrefix(k, data.MetadataPrefix) && !strings.HasPrefix(k, "$") {
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

	return keys
}

// Format the current symbol table flags and depth level into a string.
func tableFlagsString(s *SymbolTable, depth int) string {
	flags := fmt.Sprintf(" <level %d, id %s, ", depth, s.id.String())
	if depth < 0 {
		flags = fmt.Sprintf("<id %s", s.id.String())
	}

	if s.proxy {
		flags += "proxy, "
	}

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

	return flags
}

// Format formats a symbol table into a string for printing/display. If
// the omitPackages flag is true, it will omit the package names from
// the output.
func (s *SymbolTable) Log(session int, logger int, omitPackages bool) {
	var (
		count             int
		includingPackages string
	)

	if !ui.IsActive(logger) {
		return
	}

	name := s.Name
	if name != "" {
		name = " " + strconv.Quote(name)
	}

	// If we are omitting the packages, count the number of non-package
	// symbols in the table. Otherwise, we just report the total number
	// of symbols.
	if omitPackages {
		for _, symbol := range s.symbols {
			v := s.getValue(symbol.slot)
			if _, ok := v.(*data.Package); !ok {
				count++
			}
		}
	} else {
		count = len(s.symbols)
		includingPackages = ", including packages"
	}

	ui.Log(logger, "symbols.log.header", ui.A{
		"session": session,
		"name":    name,
		"id":      s.id.String()})

	ui.Log(logger, "symbols.log.attrs", ui.A{
		"session":  session,
		"count":    count,
		"size":     s.size,
		"packages": includingPackages})

	// Iterate over the members to get a list of the keys. Discard invisible
	// items.
	keys := make([]string, 0)

	for k := range s.symbols {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	// Now iterate over the keys in sorted order
	for _, k := range keys {
		v := s.getValue(s.symbols[k].slot)

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
			if omitPackages {
				continue
			}

			if tsx, ok := actual.Get(data.TypeMDKey); ok {
				typeString = data.String(tsx)
			} else {
				typeString = "package"
			}

		case func(*SymbolTable, []any) (any, error):
			typeString = builtinTypeName

		case func(*SymbolTable, data.List) (any, error):
			typeString = builtinTypeName

		default:
			reflectedData := fmt.Sprintf("%#v", actual)
			if strings.HasPrefix(reflectedData, "&bytecode.ByteCode") {
				typeString = funcTypeName
			}
		}

		value := data.Format(v)

		ui.Log(logger, "symbols.log.symbol", ui.A{
			"session": session,
			"name":    k,
			"type":    typeString,
			"value":   value})
	}

	if s.parent != nil {
		s.parent.Log(session, logger, omitPackages)
	}
}

// Format formats a symbol table into a string for printing/display.
func (s *SymbolTable) FormattedData(includeBuiltins bool) [][]string {
	rows := make([][]string, 0)

	// Iterate over the members to get a list of the keys. Discard invisible
	// items.
	keys := getVisibleSymbolNames(s)

	// Now iterate over the keys in sorted order
	for _, k := range keys {
		// reserved words are not valid symbol names
		if tokenizer.NewReservedToken(k).IsReserved(false) {
			continue
		}

		attr := s.symbols[k]
		v := s.getValue(attr.slot)
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
				if _, ok := k2.(func(*SymbolTable, []any) (any, error)); ok {
					hasBuiltins = true
				}
			}

			if hasBuiltins && !includeBuiltins {
				omitThisSymbol = true

				continue
			}

		case func(*SymbolTable, []any) (any, error):
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
		row[2] = data.String(attr.Readonly)
		row[3] = data.Format(v)
		rows = append(rows, row)
	}

	return rows
}
