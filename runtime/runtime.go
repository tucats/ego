// Package runtime manages the suite of builting runtime functions
// expressed as Ego packages. Each package has it's own package tree
// within the runtime folder. Each package type must provide a
// function called Initialized which is passed a symbol table, and
// registers the package functionality with the symbol table.
package runtime

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/runtime/base64"
	"github.com/tucats/ego/runtime/cipher"
	"github.com/tucats/ego/runtime/db"
	"github.com/tucats/ego/runtime/errors"
	"github.com/tucats/ego/runtime/exec"
	"github.com/tucats/ego/runtime/filepath"
	"github.com/tucats/ego/runtime/fmt"
	"github.com/tucats/ego/runtime/i18n"
	"github.com/tucats/ego/runtime/io"
	"github.com/tucats/ego/runtime/json"
	"github.com/tucats/ego/runtime/math"
	"github.com/tucats/ego/runtime/os"
	"github.com/tucats/ego/runtime/profile"
	"github.com/tucats/ego/runtime/reflect"
	"github.com/tucats/ego/runtime/rest"
	"github.com/tucats/ego/runtime/sort"
	"github.com/tucats/ego/runtime/strconv"
	"github.com/tucats/ego/runtime/strings"
	"github.com/tucats/ego/runtime/tables"
	"github.com/tucats/ego/runtime/time"
	"github.com/tucats/ego/runtime/util"
	"github.com/tucats/ego/runtime/uuid"
	"github.com/tucats/ego/symbols"
)

// AddPackages adds in the pre-defined package receivers for things like the
// table and rest runtimes.
func AddPackages(s *symbols.SymbolTable) {
	ui.Log(ui.CompilerLogger, "Adding runtime packages to %s(%v)", s.Name, s.ID())

	base64.Initialize(s)
	cipher.Initialize(s)
	db.Initialize(s)
	errors.Initialize(s)
	exec.Initialize(s)
	filepath.Initialize(s)
	fmt.Initialize(s)
	i18n.Initialize(s)
	io.Initialize(s)
	json.Initialize(s)
	math.Initialize(s)
	os.Initialize(s)
	profile.Initialize(s)
	reflect.Initialize(s)
	rest.Initialize(s)
	sort.Initialize(s)
	strconv.Initialize(s)
	strings.Initialize(s)
	tables.Initialize(s)
	time.Initialize(s)
	util.Initialize(s)
	uuid.Initialize(s)
}

// AddPackages adds in the pre-defined package receivers for things like the
// table and rest runtimes.
func AddPackage(name string, s *symbols.SymbolTable) {
	ui.Log(ui.CompilerLogger, "Adding runtime package for %s to %s(%v)", name, s.Name, s.ID())

	switch name {
	case "base64":
		base64.Initialize(s)
	case "cipher":
		cipher.Initialize(s)
	case "db":
		db.Initialize(s)
	case "errors":
		errors.Initialize(s)
	case "exec":
		exec.Initialize(s)
	case "filepath":
		filepath.Initialize(s)
	case "fmt":
		fmt.Initialize(s)
	case "i18n":
		i18n.Initialize(s)
	case "io":
		io.Initialize(s)
	case "json":
		json.Initialize(s)
	case "math":
		math.Initialize(s)
	case "os":
		os.Initialize(s)
	case "profile":
		profile.Initialize(s)
	case "reflect":
		reflect.Initialize(s)
	case "rest":
		rest.Initialize(s)
	case "sort":
		sort.Initialize(s)
	case "strconv":
		strconv.Initialize(s)
	case "strings":
		strings.Initialize(s)
	case "tables":
		tables.Initialize(s)
	case "time":
		time.Initialize(s)
	case "util":
		util.Initialize(s)
	case "uuid":
		uuid.Initialize(s)
	}
}

func TypeCompiler(t string) *data.Type {
	typeDefintion, _ := compiler.CompileTypeSpec(t, nil)

	return typeDefintion
}
