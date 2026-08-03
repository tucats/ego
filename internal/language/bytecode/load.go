package bytecode

import (
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
)

// loadByteCode instruction processor. This function loads a value
// from the symbol table and pushes it onto the stack.
//
// PERFORMANCE.md Finding 17 (see docs/internals/GLOBALS.md): if a previous
// execution of this exact instruction resolved name to one of the program's
// persistent "global singleton" tables, call Get directly on that
// remembered table -- identical semantics to c.get(name), just skipping the
// O(depth) walk through intervening call frames. On a miss, fall through to
// the unchanged c.get(name) and, if the name resolved to a global singleton,
// cache it for next time.
func loadByteCode(c *Context, i any) error {
	name := data.String(i)
	if len(name) == 0 {
		return c.runtimeError(errors.ErrInvalidIdentifier).Context(name)
	}

	if GlobalCacheEnabled {
		if table := c.bc.cachedGlobalTable(c.programCounter - 1); table != nil {
			if v, found := table.Get(name); found {
				return c.push(data.UnwrapConstant(v))
			}
		}
	}

	v, found := c.get(name)
	if !found {
		return c.runtimeError(errors.ErrUnknownIdentifier).Context(name)
	}

	if GlobalCacheEnabled {
		if table, ok := c.symbols.FindTable(name); ok && table.IsGlobalSingleton() {
			c.bc.cacheGlobalTable(c.programCounter-1, table)
		}
	}

	return c.push(data.UnwrapConstant(v))
}
