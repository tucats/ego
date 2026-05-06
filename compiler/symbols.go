package compiler

import (
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// scope tracks the set of symbols declared within a single lexical block
// (function body, if/else arm, for loop, etc.). Each scope maps symbol
// names to either nil (the symbol has been read at least once, so it is
// "used") or a non-nil *errors.Error (the symbol was declared but never
// read, so it will trigger an "unused variable" diagnostic on scope exit).
type scope struct {
	module  string            // name of the enclosing compilation unit, for error messages
	depth   int               // nesting depth at which this scope was created
	usage   map[string]*errors.Error // nil entry = used; non-nil entry = declared but not yet used
	symbols *symbols.SymbolTable
}

// The list of builtin predefined names that are always "found" during execution, and should not be
// evaluated for unresolved references during compile time.
var predefinedNames = map[string]bool{
	// builtins
	"close":   true,
	"delete":  true,
	"make":    true,
	"len":     true,
	"append":  true,
	"typeof":  true,
	"index":   true,
	"panic":   true,
	"recover": true,
	// platform/server built-ins
	"_platform": true,
	"Status":    true,
	"URL":       true,
	"Path":      true,
	// automatic  imports
	"strings": true,
	"os":      true,
	"io":      true,
	"fmt":     true,
	"sort":    true,
	"math":    true,
	"time":    true,
	// Testing infrastructure
	"T": true,
}

// newScope creates a fresh, empty scope for the given module name and nesting depth.
func newScope(name string, line int) scope {
	return scope{
		module: name,
		depth:  line,
		usage:  make(map[string]*errors.Error),
	}
}

// PushSymbolScope opens a new lexical scope. It is called whenever the compiler
// enters a new block (function body, if/else arm, loop body, etc.). Every symbol
// declared inside the block is recorded in this scope's usage map so that the
// matching PopSymbolScope can detect whether it was ever read.
func (c *Compiler) PushSymbolScope() {
	module := c.activePackageName + "." + c.b.Name()
	if module == "." {
		module = ""
	} else if module[:1] == "." {
		module = module[1:]
	}

	c.scopes = append(c.scopes, newScope(module, c.blockDepth))
}

// PopSymbolScope closes the innermost lexical scope. For each symbol that was
// declared in the scope but whose usage entry is still non-nil (meaning the
// variable was never read), an "unused variable" error is accumulated. All
// accumulated errors are chained together and returned. If the unusedVars
// flag is off, the errors are discarded and nil is returned.
func (c *Compiler) PopSymbolScope() error {
	var err *errors.Error

	pos := len(c.scopes) - 1
	if pos < 0 {
		return nil
	}

	scope := c.scopes[pos]
	for name, usageError := range scope.usage {
		if usageError != nil {
			// Is it in the "forgiven" list of known variables?
			if _, found := predefinedNames[name]; found {
				continue
			}

			if settings.GetBool(defs.UnusedVarLoggingSetting) {
				ui.Log(ui.CompilerLogger, "compiler.usage.error", ui.A{
					"name":  name,
					"error": usageError})
			}

			if errors.Nil(err) {
				err = usageError
			} else {
				err = err.Chain(usageError)
			}
		}
	}

	c.scopes = c.scopes[:pos]

	// If there was no error, or errors are suppressed, return nil.
	if err == nil || !c.flags.unusedVars {
		return nil
	}

	return err
}

// DefineSymbol registers a new symbol in the innermost scope. The usage entry
// is initially set to a non-nil "unused variable" error; it will be cleared to
// nil by the first call to ReferenceSymbol for the same name, marking the
// variable as used. Generated names (empty string, "_", or names starting with
// "$") are silently ignored because they are compiler-internal temporaries that
// the user never writes directly.
func (c *Compiler) DefineSymbol(name string) error {
	// Ignore any number of possible generated or irrelevant variable names.
	if name == "" || name == "_" || strings.HasPrefix(name, "$") {
		return nil
	}

	if len(c.scopes) == 0 {
		c.PushSymbolScope()
	}

	pos := len(c.scopes) - 1

	// Is this a previously seen undefined global variable? If so, remove
	// the reference error.
	if pos == 0 && c.symbolErrors[name] != nil {
		delete(c.symbolErrors, name)
	}

	// Look it up in the given scope
	if _, found := c.scopes[pos].usage[name]; !found {
		err := c.compileError(errors.ErrUnusedVariable).Context(name)
		c.scopes[pos].usage[name] = err

		if settings.GetBool(defs.UnusedVarLoggingSetting) {
			ui.Log(ui.CompilerLogger, "compiler.usage.create", ui.A{
				"name":     name,
				"location": err.GetLocation()})
		}
	} else if settings.GetBool(defs.UnusedVarLoggingSetting) {
		ui.Log(ui.CompilerLogger, "compiler.usage.write", ui.A{
			"name": name})
	}

	return nil
}

// Create a variable usage at the highest possible scope.
func (c *Compiler) DefineGlobalSymbol(name string) error {
	// Ignore any number of possible generated or irrelevant variable names.
	if name == "" || name == "_" || strings.HasPrefix(name, "$") {
		return nil
	}

	if len(c.scopes) == 0 {
		c.PushSymbolScope()
	}

	// Is this a previously seen undefined global variable? If so, remove
	// the reference error.
	if c.symbolErrors[name] != nil {
		delete(c.symbolErrors, name)
	}

	pos := 0
	if _, found := c.scopes[pos].usage[name]; !found {
		err := c.compileError(errors.ErrUnusedVariable).Context(name)
		c.scopes[pos].usage[name] = nil

		if settings.GetBool(defs.UnusedVarLoggingSetting) {
			ui.Log(ui.CompilerLogger, "compiler.usage.create", ui.A{
				"name":     name,
				"location": err.GetLocation()})
		}
	} else if settings.GetBool(defs.UnusedVarLoggingSetting) {
		ui.Log(ui.CompilerLogger, "compiler.usage.write", ui.A{
			"name": name})
	}

	return nil
}

// Mark a variable as being used in the current scope. If the variable has not been defined
// then an error is returned.
func (c *Compiler) ReferenceSymbol(name string) error {
	return c.validateSymbol(name, true)
}

// Mark a variable as being used. If it doesn't already exist, it is defined in the current scope.
func (c *Compiler) ReferenceOrDefineSymbol(name string) error {
	return c.validateSymbol(name, false)
}

// Validate a variable as being used in the current scope. If the variable doesn't exist, the
// mustExist flag determines whether the variable must exist or not to prevent an error.
func (c *Compiler) validateSymbol(name string, mustExist bool) error {
	var (
		err   error
		found bool
	)

	// Scan the scopes stack in reverse order and search for an entry for the
	// given variable. If found, mark it as used.
	if len(c.scopes) == 0 {
		return nil
	}

	// If its the discard variable, we don't care.
	if name == "_" {
		return nil
	}

	// Generated variable names cannot be tracked this way.
	if strings.HasPrefix(name, "$") || strings.HasPrefix(name, "__") || c.flags.trial {
		mustExist = false
	}

	root := &symbols.RootSymbolTable
	if v, found := root.Get(defs.ModeVariable); found && data.String(v) == "server" {
		mustExist = false
	}

	// Check forbidden BEFORE the scope search. Outer function locals and params
	// are still present in cx.scopes (no truncation), so without this early check
	// they would be silently found and accepted — masking the runtime isolation.
	if c.forbiddenSymbols != nil {
		if _, forbidden := c.forbiddenSymbols[name]; forbidden {
			return c.compileError(errors.ErrNestedFunctionScope).Context(name)
		}
	}

	pos := len(c.scopes) - 1

	for i := pos; i >= 0; i-- {
		if _, ok := c.scopes[i].usage[name]; ok {
			c.scopes[i].usage[name] = nil
			found = true

			if settings.GetBool(defs.UnusedVarLoggingSetting) {
				err := c.compileError(errors.ErrUnusedVariable).Context(name)

				ui.Log(ui.CompilerLogger, "compiler.usage.read", ui.A{
					"name":     name,
					"location": err.GetLocation()})
			}

			break
		}
	}

	// If the symbol wasn't ever found, check the compilation symbol table and
	// the root symbol table.
	if !found {
		err = c.resolveExternalSymbol(name, mustExist)
	}

	return err
}

// resolveExternalSymbol checks whether a name that was not found in any
// compiler scope belongs to one of the other well-known namespaces: the
// constant pool, an imported package, the predefined builtins, or the
// root symbol table. If the name is found in any of those, no error is
// returned. If mustExist is true and the name is not found anywhere, an
// ErrUnknownSymbol error is recorded for later reporting (deferred so
// that forward references inside a package can be resolved by the time
// the compilation unit closes).
func (c *Compiler) resolveExternalSymbol(name string, mustExist bool) error {
	var (
		err error
	)

	root := &symbols.RootSymbolTable

	// Is it in the constant pool? Consider found.
	if c.isConstant(name) {
		return nil
	}

	// Is it a package name or a package symbol? If os, consider found.
	if c.isPackageSymbol(name) {
		return nil
	}

	// Is it a builtin predefined name? If so, consider found.
	if _, ok := predefinedNames[name]; ok {
		return nil
	}

	// Last chance, is it in the root global symbol table?
	if _, ok := root.Get(name); ok {
		return nil
	}

	// It wasn't know to this compilation unit. If it must exist, return an error.
	if mustExist {
		err = c.compileError(errors.ErrUnknownSymbol).Context(name)
		ui.Log(ui.CompilerLogger, "compiler.usage.not.found", ui.A{
			"error": err,
			"name":  name})

		// If we don't report this stuff, never mind.
		if !settings.GetBool(defs.UnknownVarSetting) {
			err = nil
		}

		// Store this unknown symbol for later error reporting
		c.symbolErrors[name] = errors.New(err)
		err = nil
	} else {
		// If this isn't the usage where a test compilation of a fragment is being
		// performed, then even though it doesn't exist, we still want to mark it as used.
		if !c.flags.trial {
			c.DefineSymbol(name)
		}
	}

	return err
}

// isPackageSymbol returns true if name resolves to a symbol inside the
// compiler's own compile-time symbol table, the active package's symbol
// table, or the root symbol table's package dictionary. This check
// prevents spurious "unknown symbol" errors for package-qualified names
// like fmt.Println when they appear inside a package definition.
func (c *Compiler) isPackageSymbol(name string) bool {
	var found bool

	root := &symbols.RootSymbolTable

	if _, found = c.s.Get(name); !found {
		// Are we compiling in a package definition? If so, check the package table
		// to see if this is known.
		if c.activePackageName != "" {
			pkg, ok := c.packages[c.activePackageName]
			if ok {
				_, found = pkg.Get(name)
				if found {
					found = true
				}
			}
		}

		if !found {
			if v, ok := root.Get(c.activePackageName); ok {
				if p, ok := v.(*data.Package); ok {
					_, found = p.Get(name)
				}
			}
		}
	}

	return found
}

// isConstant returns true if name was declared as a constant in this
// compilation unit. Constants are always "used" by definition, so the
// compiler never reports them as unused variables.
func (c *Compiler) isConstant(name string) bool {
	found := false

	for _, constant := range c.constants {
		if constant == name {
			found = true

			break
		}
	}

	return found
}
