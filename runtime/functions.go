package runtime

import (
	"errors"
	"strings"

	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/io"
	"github.com/tucats/gopackages/expressions"
	"github.com/tucats/gopackages/symbols"
	"github.com/tucats/gopackages/util"
)

// AddBuiltinPackages adds in the pre-defined package receivers
// for things like the gremlin and rest systems.
func AddBuiltinPackages(syms *symbols.SymbolTable) {

	_ = syms.SetAlways("gremlin", map[string]interface{}{
		"New":        GremlinOpen,
		"__readonly": true,
	})
	_ = syms.SetAlways("rest", map[string]interface{}{
		"New":        RestNew,
		"Status":     RestStatusMessage,
		"__readonly": true,
	})
}

// Prompt implements the prompt() function, which uses the console
// reader
func Prompt(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	prompt := ""
	if len(args) > 0 {
		prompt = util.GetString(args[0])
	}
	text := io.ReadConsoleText(prompt)
	if text == "\n" {
		text = ""
	} else {
		if strings.HasSuffix(text, "\n") {
			text = text[:len(text)-1]
		}
	}
	return text, nil
}

// Eval implements the eval() function whcih accepts a string representation of
// an expression and returns the expression result. This can also be used to convert
// string expressions of structs or arrays
func Eval(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.New(defs.IncorrectArgumentCount)
	}
	return expressions.Evaluate(util.GetString(args[0]), symbols)
}
