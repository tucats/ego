package functions

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Printf implements fmt.printf() and is a wrapper around the native Go function.
func Printf(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	len := 0

	str, err := Sprintf(s, args)
	if errors.Nil(err) {
		len, _ = fmt.Printf("%s", util.GetString(str))
	}

	return len, err
}

// Sprintf implements fmt.sprintf() and is a wrapper around the native Go function.
func Sprintf(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) == 0 {
		return 0, nil
	}

	fmtString := util.GetString(args[0])

	if len(args) == 1 {
		return fmtString, nil
	}

	return fmt.Sprintf(fmtString, args[1:]...), nil
}

// Print implements fmt.Print() and is a wrapper around the native Go function.
func Print(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var b strings.Builder

	for i, v := range args {
		if i > 0 {
			b.WriteString(" ")
		}

		b.WriteString(FormatAsString(s, v))
	}

	text, e2 := fmt.Printf("%s", b.String())

	return text, errors.New(e2)
}

// Println implements fmt.Println() and is a wrapper around the native Go function.
func Println(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var b strings.Builder

	for i, v := range args {
		if i > 0 {
			b.WriteString(" ")
		}

		b.WriteString(FormatAsString(s, v))
	}

	text, e2 := fmt.Printf("%s\n", b.String())

	return text, errors.New(e2)
}

// UseString will attempt to use the String() function of the
// object passed in, if it is a typed struct.  Otherwise, it
// just returns the Unquoted format value.
func FormatAsString(s *symbols.SymbolTable, v interface{}) string {
	if m, ok := v.(map[string]interface{}); ok {
		if f, ok := m["String"]; ok && f != nil {
			if fmt, ok := f.(func(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError)); ok {
				local := symbols.NewChildSymbolTable("local to format", s)
				_ = local.SetAlways("__this", v)

				if si, err := fmt(local, []interface{}{}); err == nil {
					if str, ok := si.(string); ok {
						return str
					}
				}
			}
		}
	}

	return util.FormatUnquoted(v)
}
