package expressions

import (
	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Eval evaluates the parsed expression. This can be called multiple times
// with the same compiled expression, but with different symbols. The function
// returns the exprssion value as computed by the compiled expression code,
// and an error value that is nil if no errors occurred.
func (e *Expression) Eval(s *symbols.SymbolTable) (interface{}, error) {
	// If the compile failed, bail out now.
	if e.err != nil {
		return nil, e.err
	}

	// If the symbol table we're given is unallocated, make one for our use now.
	if s == nil {
		s = symbols.NewSymbolTable("eval")
	}

	// Add the builtin functions
	builtins.AddBuiltins(s)

	// Run the generated code to get a result
	ctx := bytecode.NewContext(s, e.b).SetExtensions(true)

	err := ctx.Run()
	if err != nil && !errors.Equals(err, errors.ErrStop) {
		return nil, err
	}

	return ctx.Pop()
}
