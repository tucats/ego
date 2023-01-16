package runtime

import (
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/expressions"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/runtime/command"
	"github.com/tucats/ego/runtime/db"
	"github.com/tucats/ego/runtime/rest"
	"github.com/tucats/ego/runtime/table"
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
	ui.Log(ui.CompilerLogger, "Adding runtime packages to %s(%v)", s.Name, s.ID())

	s.SetAlways("exec", data.NewPackageFromMap("exec", map[string]interface{}{
		"Command":          command.Command,
		"LookPath":         command.LookPath,
		data.TypeMDKey:     data.PackageType("exec"),
		data.ReadonlyMDKey: true,
	}))

	s.SetAlways("rest", data.NewPackageFromMap("rest", map[string]interface{}{
		"New":              rest.New,
		"Status":           rest.Status,
		"ParseURL":         rest.ParseURL,
		data.TypeMDKey:     data.PackageType("rest"),
		data.ReadonlyMDKey: true,
	}))

	s.SetAlways("db", data.NewPackageFromMap("db", map[string]interface{}{
		"New":              db.New,
		data.TypeMDKey:     data.PackageType("db"),
		data.ReadonlyMDKey: true,
	}))

	var utilPkg *data.Package

	utilV, found := s.Root().Get("util")
	if !found {
		utilPkg, _ = bytecode.GetPackage("util")
	} else {
		utilPkg = utilV.(*data.Package)
	}

	utilPkg.Set("SymbolTables", SymbolTables)
	_ = s.Root().SetWithAttributes("util", utilPkg, symbols.SymbolAttribute{Readonly: true})

	s.SetAlways("tables", data.NewPackageFromMap("tables", map[string]interface{}{
		"New":              table.New,
		data.TypeMDKey:     data.PackageType("tables"),
		data.ReadonlyMDKey: true,
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
func Prompt(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 1 {
		return nil, errors.ErrArgumentCount
	}

	prompt := ""
	if len(args) > 0 {
		prompt = data.String(args[0])
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
func Eval(symbols *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 1 {
		return nil, errors.ErrArgumentCount
	}

	return expressions.Evaluate(data.String(args[0]), symbols)
}

func GetDeclaration(fname string) *data.FunctionDeclaration {
	if fname == "" {
		return nil
	}

	fd, ok := functions.FunctionDictionary[fname]
	if ok {
		return fd.D
	}

	return nil
}

func TypeCompiler(t string) *data.Type {
	typeDefintion, _ := compiler.CompileTypeSpec(t)

	return typeDefintion
}
