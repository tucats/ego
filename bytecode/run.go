package bytecode

import (
	"fmt"
	"sync/atomic"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

// growStackBy indicates the number of elements to add to the stack when
// it runs out of space.
const growStackBy = 50

func (c *Context) StepOver(b bool) {
	c.stepOver = b
}

// Run executes a bytecode context.
func (c *Context) Run() error {
	return c.RunFromAddress(0)
}

// Used to resume execution after an event like the debugger being invoked.
func (c *Context) Resume() error {
	return c.RunFromAddress(c.programCounter)
}

// IsRunning returns true if the context is still executing instructions.
func (c *Context) IsRunning() bool {
	return c.running
}

// RunFromAddress executes a bytecode context from a given starting address.
func (c *Context) RunFromAddress(addr int) error {
	var err error

	// Reset the runtime context.
	c.programCounter = addr
	c.running = true

	ui.Log(ui.TraceLogger, "trace.tracing", ui.A{
		"name":   c.name,
		"thread": c.threadID})

	// Loop over the bytecodes and run.
	for c.running && c.programCounter < len(c.bc.instructions) {
		i := c.bc.instructions[c.programCounter]
		if c.Tracing() {
			traceInstruction(c, i)
		}

		c.programCounter = c.programCounter + 1

		imp := dispatchTable[i.Operation]
		if imp == nil {
			continue
		}

		atomic.AddInt64(&InstructionsExecuted, 1)

		// Call the implementation of the opcode, and handle any try/catch processing that
		// results from the execution. The result of handleCatch is the error state AFTER
		// any try/catch block branching has been done.
		err = handleCatch(c, imp(c, i.Operand))
		if err != nil {
			// If it's a panic, format the error, print call frames, and stop execution.
			if errors.Equals(err, errors.ErrPanic) {
				fmt.Printf("Error: %v\n", err)
				fmt.Print(c.FormatFrames(OmitSymbolTableNames))

				err = errors.ErrStop
			}

			// If it's not a flow-of-control signal, trace the error.
			if !errors.Equals(err, errors.ErrSignalDebugger) && !errors.Equals(err, errors.ErrStop) {
				ui.Log(ui.TraceLogger, "trace.return", ui.A{
					"thread": c.threadID,
					"error":  err})
			}

			if err != nil {
				err = errors.New(err)
			}

			return err
		}
	}

	ui.Log(ui.TraceLogger, "trace.end", ui.A{
		"name":   c.name,
		"thread": c.threadID})

	// If we ended successfully, but a go routine we started failed with an error, let's
	// report that as our error state.
	if err == nil && c.goErr != nil {
		err = c.goErr
		c.goErr = nil
	}

	if err != nil {
		return errors.New(err)
	}

	return nil
}
