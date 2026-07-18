package resolve

import "github.com/tucats/ego/internal/language/parse/ast"

// DiagnosticKind categorizes a Diagnostic.
type DiagnosticKind int

const (
	// DiagnosticInvalid is the zero value and marks an uninitialized Diagnostic.
	DiagnosticInvalid DiagnosticKind = iota

	// UnusedSymbol reports a name that was declared in some scope but never
	// read anywhere in that scope's extent.
	UnusedSymbol

	// UndefinedSymbol reports an identifier reference that could not be
	// resolved to any declaration, import, or entry in the builtin allow-list
	// (see builtins.go), and is not a forward reference to a same-function
	// local declared later (see UsedBeforeDefined).
	UndefinedSymbol

	// UsedBeforeDefined reports an identifier reference that resolves to a
	// local declared later in the same function -- a specific, higher-value
	// case of an otherwise-unresolved reference.
	UsedBeforeDefined
)

// diagnosticKindNames doubles as a localization-key table: each value is
// spelled as a dotted key in this repo's established i18n convention (see
// CLAUDE.md, e.g. "compiler.usage.error", "log.server.request") rather than
// an arbitrary display string, so a consumer that wants localized/formatted
// messages -- a canonical formatter surfacing Resolve's findings, a linter
// CLI -- can use DiagnosticKind.String() directly as the lookup key into its
// own message catalog. This package deliberately does not import
// internal/errors/internal/i18n itself (see the package doc: resolve stays a
// standalone, decoupled consumer of ast, not of the compiler's
// infrastructure), so Diagnostic.Message remains a plain, already-formatted
// English fallback for callers that don't wire up a catalog at all.
var diagnosticKindNames = map[DiagnosticKind]string{
	DiagnosticInvalid: "resolve.diagnostic.invalid",
	UnusedSymbol:      "resolve.symbol.unused",
	UndefinedSymbol:   "resolve.symbol.undefined",
	UsedBeforeDefined: "resolve.symbol.used.before.defined",
}

// String returns the diagnostic kind's localization key (see
// diagnosticKindNames).
func (k DiagnosticKind) String() string {
	if name, ok := diagnosticKindNames[k]; ok {
		return name
	}

	return "resolve.diagnostic.unknown"
}

// Diagnostic reports one violation found during Resolve: an unused symbol, an
// undefined reference, or a reference used before its declaration.
type Diagnostic struct {
	// Pos is the source position the diagnostic is anchored to: the
	// declaration site for UnusedSymbol, the reference site for
	// UndefinedSymbol and UsedBeforeDefined.
	Pos ast.Position

	// Kind categorizes the diagnostic.
	Kind DiagnosticKind

	// Name is the symbol/identifier name involved.
	Name string

	// Message is a short, human-readable description.
	Message string
}
