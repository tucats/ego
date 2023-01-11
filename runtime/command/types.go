package command

import (
	"sync"

	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
)

// exec.Cmd type specification.
const commandTypeSpec = `
	type exec.Cmd struct {
		__cmd       interface{},
		Dir         string,
		Path		string,
		Args		[]string,
		Env			[]string,
		Stdout      []string,
		Stdin       []string,
	}`

var commandTypeDef *data.Type
var commandTypeDefLock sync.Mutex

func initCommandTypeDef() {
	commandTypeDefLock.Lock()
	defer commandTypeDefLock.Unlock()

	if commandTypeDef == nil {
		t, _ := compiler.CompileTypeSpec(commandTypeSpec)

		t.DefineFunctions(map[string]data.Function{
			"Output": {Value: Output},
			"Run":    {Value: Run},
		})

		commandTypeDef = t
	}
}
