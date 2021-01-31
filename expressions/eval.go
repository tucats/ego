package expressions

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
)

// Eval evaluates the parsed expression. This can be called multiple times
// with the same scanned string, but with different symbols.
func (e *Expression) Eval(s *symbols.SymbolTable) (interface{}, error) {

	// If the compile failed, bail out now.
	if e.err != nil {
		return nil, e.err
	}

	// If the symbol table we're given is unallocated, make one for our use now.
	if s == nil {
		s = symbols.NewSymbolTable("eval()")

	}

	// Add the builtin functions
	functions.AddBuiltins(s)

	// Run the generated code to get a result
	ctx := bytecode.NewContext(s, e.b)
	err := ctx.Run()
	if err != nil {
		if err.Error() != "stop" {
			return nil, err
		}
	}

	return ctx.Pop()
}
