package exec

import (
	"sync"

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
var commandTypeDefLock sync.Mutex

func InitializeExec(s *symbols.SymbolTable) {
	commandTypeDefLock.Lock()
	defer commandTypeDefLock.Unlock()

	if commandTypeDef == nil {
		t, _ := compiler.CompileTypeSpec(commandTypeSpec)

		t.DefineFunctions(map[string]data.Function{
			"Output": {Value: Output},
			"Run":    {Value: Run},
		})

		commandTypeDef = t.SetPackage("exec")

		s.SharedParent().SetAlways("exec", data.NewPackageFromMap("exec", map[string]interface{}{
			"Command":          Command,
			"LookPath":         LookPath,
			"Cmd":              t,
			data.TypeMDKey:     data.PackageType("exec"),
			data.ReadonlyMDKey: true,
		}))
	}
}
