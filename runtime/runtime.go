// Package runtime manages the suite of built-in runtime functions
// expressed as Ego packages. Each package has it's own package tree
// within the runtime folder. Each package type must provide a
// function called Initialized which is passed a symbol table, and
// registers the package functionality with the symbol table.
package runtime

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/packages"
	"github.com/tucats/ego/runtime/base64"
	"github.com/tucats/ego/runtime/cipher"
	"github.com/tucats/ego/runtime/db"
	"github.com/tucats/ego/runtime/errors"
	"github.com/tucats/ego/runtime/exec"
	"github.com/tucats/ego/runtime/filepath"
	"github.com/tucats/ego/runtime/fmt"
	"github.com/tucats/ego/runtime/http"
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
	"github.com/tucats/ego/runtime/sync"
	"github.com/tucats/ego/runtime/tables"
	"github.com/tucats/ego/runtime/time"
	"github.com/tucats/ego/runtime/util"
	"github.com/tucats/ego/runtime/uuid"
	"github.com/tucats/ego/symbols"
)

// AddPackages adds in the pre-defined package receivers to the given symbol
// table. Note that these packages _must_ hav already been imported.
func AddPackages(s *symbols.SymbolTable) {
	ui.Log(ui.PackageLogger, "pkg.runtime.packages", ui.A{
		"name": s.Name,
		"id":   s.ID()})

	for _, name := range []string{
		"base64",
		"cipher",
		"db",
		"errors",
		"exec",
		"filepath",
		"fmt",
		"i18n",
		"io",
		"json",
		"math",
		"os",
		"profile",
		"reflect",
		"rest",
		"sort",
		"strconv",
		"strings",
		"sync",
		"tables",
		"time",
		"util",
		"uuid",
	} {
		pkg := packages.Get(name)
		if pkg == nil {
			ui.Log(ui.InternalLogger, "pkg.runtime.packages.missing", ui.A{
				"name": name})

			continue
		}

		s.SetAlways(name, pkg)
	}
}

// AddPackages adds in the pre-defined package receivers for things like the
// table and rest runtimes.
func AddPackage(name string) *data.Package {
	var (
		p *data.Package
	)

	s := symbols.NewRootSymbolTable(name)

	ui.Log(ui.PackageLogger, "pkg.runtime.packages", ui.A{
		"package": name,
		"name":    s.Name,
		"id":      s.ID()})

	switch name {
	case "base64":
		p = base64.Base64Package

	case "cipher":
		p = cipher.CipherPackage

	case "db":
		p = db.DBPackage

	case "errors":
		p = errors.ErrorsPackage

	case "exec":
		p = exec.ExecPackage

	case "filepath":
		p = filepath.FilepathPackage

	case "fmt":
		p = fmt.FmtPackage

	case "http":
		p = http.HttpPackage

	case "i18n":
		p = i18n.I18nPackage

	case "io":
		p = io.IoPackage

	case "json":
		p = json.JsonPackage

	case "math":
		p = math.MathPackage

	case "os":
		p = os.OsPackage

	case "profile":
		p = profile.ProfilePackage

	case "reflect":
		p = reflect.ReflectPackage

	case "rest":
		p = rest.RestPackage

	case "sort":
		p = sort.SortPackage

	case "strconv":
		p = strconv.StrconvPackage

	case "strings":
		p = strings.StringsPackage

	case "sync":
		p = sync.SyncPackage

	case "tables":
		p = tables.TablesPackage

	case "time":
		p = time.TimePackage

	case "util":
		p = util.UtilPackage

	case "uuid":
		p = uuid.UUIDPackage

	default:
		ui.Log(ui.PackageLogger, "pkg.runtime.unknown", ui.A{
			"name": name})
	}

	if p != nil {
		packages.Save(p)
	}

	return p
}
