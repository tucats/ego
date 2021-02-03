package functions

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Printf implements fmt.printf() and is a wrapper around the native Go function
func Printf(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	str, err := Sprintf(s, args)
	len := 0
	if err == nil {
		len, _ = fmt.Printf("%s", util.GetString(str))
	}

	return len, err
}

// Sprintf implements fmt.sprintf() and is a wrapper around the native Go function
func Sprintf(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) == 0 {
		return 0, nil
	}
	fmtString := util.GetString(args[0])
	if len(args) == 1 {
		return fmtString, nil
	}

	return fmt.Sprintf(fmtString, args[1:]...), nil
}

// Print implements fmt.Print() and is a wrapper around the native Go function
func Print(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var b strings.Builder

	for i, v := range args {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(util.FormatUnquoted(v))
	}

	return fmt.Printf("%s", b.String())
}

// Println implements fmt.Println() and is a wrapper around the native Go function
func Println(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	var b strings.Builder

	for i, v := range args {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(util.FormatUnquoted(v))
	}

	return fmt.Printf("%s\n", b.String())
}
