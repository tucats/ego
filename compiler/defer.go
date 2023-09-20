package compiler

import (
	"strconv"
	"sync/atomic"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

var deferSequence int32 = 0

// compileDefer compiles the "defer" statement. This compiles a statement,
// and attaches the resulting bytecode to the compilation unit's defer queue.
// Later, when a return is processed, this queue will be used to generate the
// appropriate deferred operations. The order of the "defer" statements determines
// the order in the queue, and therefore the order in which they are run when a
// return is executed.
func (c *Compiler) compileDefer() error {
	// Make sure there is an actual following statement.
	if c.t.AnyNext(tokenizer.SemicolonToken, tokenizer.EndOfTokens) {
		return c.error(errors.ErrMissingStatement)
	}

	// Emit code indicating that this defer statement has been executed. This allows
	// runtime detection of whether a defer statement has been executed or not when
	// the function exits. Each defer stateemnt gets a unique identifier name, so there
	// can be no collision in defer statements in nested functions.
	atomic.AddInt32(&deferSequence, 1)

	// Create a deferred statement object to hold the address of the statement
	// and a unique name for the defer statement.
	ds := deferStatement{Name: "__defer_" + strconv.Itoa(int(deferSequence))}

	c.b.Emit(bytecode.Push, true)
	c.b.Emit(bytecode.StoreGlobal, ds.Name)

	// Branch around this block of code that will contain the defer statement.
	start := c.b.Mark()
	c.b.Emit(bytecode.Branch, 0)

	// We are at the start of the defer statement, so it's time to store the target
	// address of the LocalCall that will invoke the statement.
	ds.Address = c.b.Mark()

	// Compile the defer statement, and ensure it ends with
	// a Return operation. This address is then stored in the
	// defer queue, indicating the address(es) of defer statements
	// to be executed as local calls before a return from this
	// block.
	err := c.compileStatement()
	if err == nil {
		c.b.Emit(bytecode.Return)

		c.deferQueue = append(c.deferQueue, ds)
		err = c.b.SetAddressHere(start)
	}

	return err
}
