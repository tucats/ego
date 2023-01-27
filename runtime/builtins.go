package runtime

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/runtime/db"
	"github.com/tucats/ego/runtime/errors"
	"github.com/tucats/ego/runtime/exec"
	"github.com/tucats/ego/runtime/io"
	"github.com/tucats/ego/runtime/os"
	"github.com/tucats/ego/runtime/rest"
	"github.com/tucats/ego/runtime/sort"
	"github.com/tucats/ego/runtime/tables"
	"github.com/tucats/ego/runtime/util"
	"github.com/tucats/ego/runtime/uuid"
	"github.com/tucats/ego/symbols"
)

// AddBuiltinPackages adds in the pre-defined package receivers
// for things like the table and rest systems.
func AddBuiltinPackages(s *symbols.SymbolTable) {
	ui.Log(ui.CompilerLogger, "Adding runtime packages to %s(%v)", s.Name, s.ID())

	errors.InitializeErrors(s)
	exec.Initialize(s)
	db.Initialize(s)
	rest.Initialize(s)
	sort.Initialize(s)
	tables.Initialize(s)
	io.Initialize(s)
	os.Initialize(s)
	util.Initialize(s)
	uuid.Initialize(s)
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
