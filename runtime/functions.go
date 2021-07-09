package runtime

import (
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/expressions"
	"github.com/tucats/ego/functions"
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
func AddBuiltinPackages(s *symbols.SymbolTable) {
	ui.Debug(ui.CompilerLogger, "Adding runtime packages to %s(%v)", s.Name, s.ID)

	/* Add only if you have enabled the gremlin build tag.
	_ = s.SetAlways("gremlin", datatypes.EgoPackage{
		"New":                   GremlinOpen,
		datatypes.TypeMDKey:     datatypes.Package("gremlin"),
		datatypes.ReadonlyMDKey: true,
	})
	*/

	_ = s.SetAlways("rest", datatypes.EgoPackage{
		"New":                   RestNew,
		"Status":                RestStatusMessage,
		datatypes.TypeMDKey:     datatypes.Package("rest"),
		datatypes.ReadonlyMDKey: true,
	})

	_ = s.SetAlways("db", datatypes.EgoPackage{
		"New":                   DBNew,
		datatypes.TypeMDKey:     datatypes.Package("db"),
		datatypes.ReadonlyMDKey: true,
	})

	_ = s.SetAlways("tables", datatypes.EgoPackage{
		"New":                   TableNew,
		datatypes.TypeMDKey:     datatypes.Package("tables"),
		datatypes.ReadonlyMDKey: true,
	})

	// Add the sort.Slice function, which must live outside
	// the function package to avoid import cycles.
	_ = functions.AddFunction(s, functions.FunctionDefinition{
		Name:      "Slice",
		Pkg:       "sort",
		Min:       2,
		Max:       2,
		FullScope: true,
		F:         sortSlice,
	})

	_ = functions.AddFunction(s, functions.FunctionDefinition{
		Name:      "Symbols",
		Pkg:       "util",
		Min:       0,
		Max:       2,
		FullScope: true,
		F:         FormatSymbols,
	})
}

// Prompt implements the prompt() function, which uses the console
// reader.
func Prompt(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
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
		text = strings.TrimSuffix(text, "\n")
	}

	return text, nil
}

// Eval implements the eval() function which accepts a string representation of
// an expression and returns the expression result. This can also be used to convert
// string expressions of structs or arrays.
func Eval(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	return expressions.Evaluate(util.GetString(args[0]), symbols)
}
