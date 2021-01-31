package runtime

import (
	"errors"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/expressions"
	"github.com/tucats/ego/io"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// passwordPromptPrefix is the string prefix you can put in the prompt
// string for a call to the Ego prompt() function to cause it to suppress
// keyboard echo for the input. The text after this prefix, if any, is used
// as the prompt text.
const passwordPromptPrefix = "password~"

// AddBuiltinPackages adds in the pre-defined package receivers
// for things like the gremlin and rest systems.
func AddBuiltinPackages(syms *symbols.SymbolTable) {

	_ = syms.SetAlways("gremlin", map[string]interface{}{
		"New": GremlinOpen,
		datatypes.MetadataKey: map[string]interface{}{
			datatypes.TypeMDKey:     "package",
			datatypes.ReadonlyMDKey: true,
		},
	})
	_ = syms.SetAlways("rest", map[string]interface{}{
		"New":    RestNew,
		"Status": RestStatusMessage,
		datatypes.MetadataKey: map[string]interface{}{
			datatypes.TypeMDKey:     "package",
			datatypes.ReadonlyMDKey: true,
		},
	})
	_ = syms.SetAlways("db", map[string]interface{}{
		"New": DBNew,
		datatypes.MetadataKey: map[string]interface{}{
			datatypes.TypeMDKey:     "package",
			datatypes.ReadonlyMDKey: true,
		},
	})
	_ = syms.SetAlways("tables", map[string]interface{}{
		"New": TableNew,
		datatypes.MetadataKey: map[string]interface{}{
			datatypes.TypeMDKey:     "package",
			datatypes.ReadonlyMDKey: true,
		},
	})
}

// Prompt implements the prompt() function, which uses the console
// reader
func Prompt(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	prompt := ""
	if len(args) > 0 {
		prompt = util.GetString(args[0])
	}

	var text string
	if strings.HasPrefix(prompt, passwordPromptPrefix) {
		text = ui.PromptPassword(prompt[len(passwordPromptPrefix):])
	} else {
		text = io.ReadConsoleText(prompt)
	}

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
