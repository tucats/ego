package runtime

import (
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/expressions"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
)

// passwordPromptPrefix is the string prefix you can put in the prompt
// string for a call to the Ego prompt() function to cause it to suppress
// keyboard echo for the input. The text after this prefix, if any, is used
// as the prompt text.
const passwordPromptPrefix = "password~"

// AddBuiltinPackages adds in the pre-defined package receivers
// for things like the table and rest systems.
func AddBuiltinPackages(s *symbols.SymbolTable) {
	ui.Debug(ui.CompilerLogger, "Adding runtime packages to %s(%v)", s.Name, s.ID)

	_ = s.SetAlways("exec", datatypes.NewPackageFromMap("exec", map[string]interface{}{
		"Command":               NewCommand,
		"LookPath":              LookPath,
		datatypes.TypeMDKey:     datatypes.Package("exec"),
		datatypes.ReadonlyMDKey: true,
	}))

	_ = s.SetAlways("rest", datatypes.NewPackageFromMap("rest", map[string]interface{}{
		"New":                   RestNew,
		"Status":                RestStatusMessage,
		"ParseURL":              RestParseURL,
		datatypes.TypeMDKey:     datatypes.Package("rest"),
		datatypes.ReadonlyMDKey: true,
	}))

	_ = s.SetAlways("db", datatypes.NewPackageFromMap("db", map[string]interface{}{
		"New":                   DBNew,
		datatypes.TypeMDKey:     datatypes.Package("db"),
		datatypes.ReadonlyMDKey: true,
	}))

	var utilPkg *datatypes.EgoPackage

	utilV, found := s.Root().Get("util")
	if !found {
		utilPkg, _ = bytecode.GetPackage("util")
	} else {
		utilPkg = utilV.(*datatypes.EgoPackage)
	}

	utilPkg.Set("SymbolTables", SymbolTables)
	_ = s.Root().SetWithAttributes("util", utilPkg, symbols.SymbolAttribute{Readonly: true})

	_ = s.SetAlways("tables", datatypes.NewPackageFromMap("tables", map[string]interface{}{
		"New":                   TableNew,
		datatypes.TypeMDKey:     datatypes.Package("tables"),
		datatypes.ReadonlyMDKey: true,
	}))

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

	_ = functions.AddFunction(s, functions.FunctionDefinition{
		Name:      "Prompt",
		Pkg:       "io",
		Min:       0,
		Max:       1,
		FullScope: true,
		F:         Prompt,
	})

	_ = functions.AddFunction(s, functions.FunctionDefinition{
		Name:      "Eval",
		Pkg:       "util",
		Min:       1,
		Max:       1,
		FullScope: true,
		F:         Eval,
	})
}

// Prompt implements the prompt() function, which uses the console
// reader.
func Prompt(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	prompt := ""
	if len(args) > 0 {
		prompt = datatypes.GetString(args[0])
	}

	var text string
	if strings.HasPrefix(prompt, passwordPromptPrefix) {
		text = ui.PromptPassword(prompt[len(passwordPromptPrefix):])
	} else {
		text = ReadConsoleText(prompt)
	}

	text = strings.TrimSuffix(text, "\n")

	return text, nil
}

// Eval implements the eval() function which accepts a string representation of
// an expression and returns the expression result. This can also be used to convert
// string expressions of structs or arrays.
func Eval(symbols *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) != 1 {
		return nil, errors.New(errors.ErrArgumentCount)
	}

	return expressions.Evaluate(datatypes.GetString(args[0]), symbols)
}

func GetDeclaration(fname string) *datatypes.FunctionDeclaration {
	if fname == "" {
		return nil
	}

	fd, ok := functions.FunctionDictionary[fname]
	if ok {
		return fd.D
	}

	return nil
}
