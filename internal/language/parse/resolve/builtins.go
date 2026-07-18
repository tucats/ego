package resolve

// predefinedNames lists identifiers this package treats as always resolvable,
// even though no declaration for them appears anywhere in a given
// *ast.File: Ego's always-available builtin functions, a handful of
// platform/server built-ins, and the package names auto-imported for `ego
// test` compilations (see CLAUDE.md, "Auto-imported packages"). This list is
// deliberately a best-effort duplicate of
// internal/language/compiler/symbols.go's predefinedNames map (not an
// import of it -- this package intentionally does not depend on
// internal/language/compiler) plus the auto-import package set, and is not
// exhaustive: it exists to keep UndefinedSymbol diagnostics conservative on
// common real-world source, not to fully replicate the compiler's dynamic,
// settings-dependent name resolution (ego.compiler.import,
// ego.compiler.extensions, loaded runtime packages, etc.). See the package
// doc's "Known limitations" section.
var predefinedNames = map[string]bool{
	// Always-available builtins.
	"close":   true,
	"delete":  true,
	"make":    true,
	"len":     true,
	"append":  true,
	"typeof":  true,
	"sizeof":  true,
	"index":   true,
	"panic":   true,
	"recover": true,
	"complex": true,
	"real":    true,
	"imag":    true,

	// Platform/server built-ins.
	"_platform": true,
	"Status":    true,
	"URL":       true,
	"Path":      true,

	// Testing infrastructure.
	"T": true,

	// Packages auto-imported for `ego test` compilations.
	"base64":   true,
	"cipher":   true,
	"cmplx":    true,
	"errors":   true,
	"exec":     true,
	"filepath": true,
	"fmt":      true,
	"io":       true,
	"json":     true,
	"math":     true,
	"os":       true,
	"profile":  true,
	"reflect":  true,
	"rest":     true,
	"sort":     true,
	"sql":      true,
	"strconv":  true,
	"strings":  true,
	"tables":   true,
	"time":     true,
	"util":     true,
	"uuid":     true,
}

// isPredefined reports whether name should be treated as always resolvable,
// bypassing UndefinedSymbol/UsedBeforeDefined diagnostics.
func isPredefined(name string) bool {
	return predefinedNames[name]
}
