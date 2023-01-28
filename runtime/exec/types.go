package exec

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

// exec.Cmd type specification.
const commandTypeSpec = `
	type Cmd struct {
		cmd         interface{},
		Dir         string,
		Path		string,
		Args		[]string,
		Env			[]string,
		Stdout      []string,
		Stdin       []string,
	}`

var commandTypeDef *data.Type

func Initialize(s *symbols.SymbolTable) {
	t, _ := compiler.CompileTypeSpec(commandTypeSpec)

	t.DefineFunctions(map[string]data.Function{
		"Output": {Value: output},
		"Run":    {Value: run},
	})

	commandTypeDef = t.SetPackage("exec")

	newpkg := data.NewPackageFromMap("exec", map[string]interface{}{
		"Command": data.Function{
			Declaration: &data.Declaration{
				Name: "Command",
				Parameters: []data.Parameter{
					{
						Name: "cmd",
						Type: data.StringType,
					},
				},
				Returns:  []*data.Type{t},
				Variadic: true,
			},
			Value: newCommand,
		},
		"LookPath": data.Function{
			Declaration: &data.Declaration{
				Name: "LookPath",
				Parameters: []data.Parameter{
					{
						Name: "file",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.StringType, data.ErrorType},
			},
			Value: lookPath,
		},
		"Cmd": t,
	}).SetBuiltins(true)

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
