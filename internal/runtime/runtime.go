// Package runtime manages the suite of built-in runtime functions
// expressed as Ego packages. Each package has it's own package tree
// within the runtime folder. Each package type must provide a
// function called Initialized which is passed a symbol table, and
// registers the package functionality with the symbol table.
package runtime

import (
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/packages"
	"github.com/tucats/ego/internal/runtime/base64"
	"github.com/tucats/ego/internal/runtime/cipher"
	"github.com/tucats/ego/internal/runtime/errors"
	"github.com/tucats/ego/internal/runtime/exec"
	"github.com/tucats/ego/internal/runtime/filepath"
	"github.com/tucats/ego/internal/runtime/fmt"
	"github.com/tucats/ego/internal/runtime/http"
	"github.com/tucats/ego/internal/runtime/i18n"
	"github.com/tucats/ego/internal/runtime/io"
	"github.com/tucats/ego/internal/runtime/json"
	"github.com/tucats/ego/internal/runtime/math"
	"github.com/tucats/ego/internal/runtime/os"
	"github.com/tucats/ego/internal/runtime/profile"
	"github.com/tucats/ego/internal/runtime/proxy"
	"github.com/tucats/ego/internal/runtime/reflect"
	"github.com/tucats/ego/internal/runtime/rest"
	"github.com/tucats/ego/internal/runtime/runtime"
	"github.com/tucats/ego/internal/runtime/sort"
	"github.com/tucats/ego/internal/runtime/sql"
	"github.com/tucats/ego/internal/runtime/strconv"
	"github.com/tucats/ego/internal/runtime/strings"
	"github.com/tucats/ego/internal/runtime/sync"
	"github.com/tucats/ego/internal/runtime/tables"
	"github.com/tucats/ego/internal/runtime/time"
	"github.com/tucats/ego/internal/runtime/util"
	"github.com/tucats/ego/internal/runtime/uuid"
	"github.com/tucats/ego/internal/language/symbols"
)

// AddPackages adds in the pre-defined package receivers to the given symbol
// table. Note that these packages _must_ hav already been imported.
func AddPackages(s *symbols.SymbolTable) {
	if ui.IsActive(ui.PackageLogger) {
		ui.Log(ui.PackageLogger, "pkg.runtime.packages", ui.A{
			"name": s.Name,
			"id":   s.ID()})
	}

	for _, name := range []string{
		"base64",
		"cipher",
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
		"proxy",
		"reflect",
		"rest",
		"runtime",
		"sort",
		"sql",
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
			if ui.IsActive(ui.InternalLogger) {
				ui.Log(ui.InternalLogger, "pkg.runtime.packages.missing", ui.A{
					"name": name})
			}

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

	if ui.IsActive(ui.PackageLogger) {
		ui.Log(ui.PackageLogger, "pkg.runtime.packages", ui.A{
			"package": name,
			"name":    s.Name,
			"id":      s.ID()})
	}

	switch name {
	case "base64":
		p = base64.Base64Package

	case "cipher":
		p = cipher.CipherPackage

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

	case "proxy":
		p = proxy.ProxyPackage

	case "reflect":
		p = reflect.ReflectPackage

	case "rest":
		p = rest.RestPackage

	case "runtime":
		p = runtime.RuntimePackage

	case "sort":
		p = sort.SortPackage

	case "strconv":
		p = strconv.StrconvPackage

	case "strings":
		p = strings.StringsPackage

	case "sql":
		p = sql.SqlPackage

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
		if ui.IsActive(ui.PackageLogger) {
			ui.Log(ui.PackageLogger, "pkg.runtime.unknown", ui.A{
				"name": name})
		}
	}

	if p != nil {
		packages.Save(p)
	}

	return p
}
