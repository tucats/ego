package bytecode

import (
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/errors"
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
// This can be stopped explicitly by clearing the running flag, or by
// attempting to execute past the end of the bytecode.
func (c *Context) IsRunning() bool {
	return c.running.Load() && (c.programCounter < c.bc.Size())
}

// RunFromAddress executes a bytecode context from a given starting address.
func (c *Context) RunFromAddress(addr int) error {
	var err error

	// Reset the runtime context.
	c.programCounter = addr
	c.running.Store(true)

	if ui.IsActive(ui.TraceLogger) {
		ui.Log(ui.TraceLogger, "trace.tracing", ui.A{
			"name":   c.name,
			"thread": c.threadID})
	}

	// Set a SIGINT trap so a user pressing Ctrl-C stops execution.
	//
	// GORTNS-1: this used to leak one goroutine on every single call, and the
	// reason is a subtle property of os/signal that is worth spelling out.
	//
	// The watcher goroutine below blocks on "<-intChan" waiting for a signal.
	// The cleanup used to be nothing but "signal.Stop(intChan)". Stop does what
	// its documentation says -- it stops the signal package from *delivering*
	// any more signals to that channel -- but it does NOT close the channel.
	// A receive on an open channel that nobody will ever send to blocks forever,
	// so the watcher never returned. Because it captures c, each abandoned
	// goroutine also kept an entire *Context alive (symbol table, stack,
	// bytecode), making this a memory leak as well as a goroutine leak.
	//
	// This function is called once per bytecode execution, which made the leak
	// grow without bound in the worst places:
	//
	//   - once per HTTP service request in the server;
	//   - once per comparison inside sort.Slice with an Ego comparator, so a
	//     single sort of 300 elements leaked roughly 2500 goroutines;
	//   - once per value formatted by fmt when the value has an Ego String()
	//     method.
	//
	// The fix is the standard Go "done channel" shutdown pattern. We create a
	// second channel used purely as a broadcast signal, and the watcher waits on
	// BOTH channels with a select. Whichever arrives first wins:
	//
	//   - a real signal arrives on intChan  -> handle the interrupt, then return;
	//   - this function finishes and closes done -> the watcher returns.
	//
	// Closing a channel is what makes this work: a receive from a closed channel
	// returns immediately rather than blocking, and it does so for every
	// receiver, so close() is the idiomatic way to broadcast "stop" to a
	// goroutine. Nothing is ever sent on done; its closure is the whole message.
	intChan := make(chan os.Signal, 1)
	signal.Notify(intChan, os.Interrupt)

	done := make(chan struct{})

	go func(c *Context) {
		select {
		case sig := <-intChan:
			// Should only ever be os.Interrupt, but just in case...
			switch sig {
			case os.Interrupt:
				if !c.interrupted.Load() && c.running.Load() {
					ui.Say(" ")
					ui.Say("msg.interrupted", ui.A{"thread": c.threadID})
				}

				if !c.debugging {
					c.running.Store(false)
				}

				c.interrupted.Store(true)

			default:
				if ui.IsActive(ui.InternalLogger) {
					ui.Log(ui.InternalLogger, "signal", ui.A{
						"thread": c.threadID,
						"signal": sig.String()})
				}
			}

		// RunFromAddress has finished, so this watcher is no longer needed.
		// Returning here is what lets the goroutine be reclaimed.
		case <-done:
		}
	}(c)

	// And, when done with this context, remove the SIGINT trap and shut the
	// watcher goroutine down. The order matters: Stop first, so no further
	// signals can be delivered to a channel nobody is listening on any more,
	// then close(done) to release the watcher.
	defer func() {
		signal.Stop(intChan)
		close(done)
	}()

	// Loop over the bytecodes and run.
	for c.running.Load() && c.programCounter < len(c.bc.instructions) {
		// Check for a break operation that needs to return to the debugger.
		if c.interrupted.Load() && c.running.Load() {
			c.running.Store(false)
			c.interrupted.Store(false)

			return errors.ErrSignalDebugger
		}

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
			// If it's a recoverable panic, begin frame-by-frame unwinding. If a
			// deferred recover() clears the panic, unwindPanic returns nil and we
			// resume; otherwise it returns ErrStop after printing the panic message.
			if errors.Equals(err, errors.ErrPanicActive) {
				err = c.unwindPanic()
				if err == nil {
					continue
				}
			}

			// If it's a fatal panic (from @fail / Panic opcode), print call frames
			// and convert to ErrStop so the run loop terminates cleanly.
			if errors.Equals(err, errors.ErrPanic) {
				fmt.Fprintf(c.output, "Error: %v\n", err)
				fmt.Fprint(c.output, c.FormatFrames(IncludeSymbolTableNames))

				err = errors.ErrStop
			}

			// If it's not a flow-of-control signal, trace the error.
			if !errors.Equals(err, errors.ErrSignalDebugger) && !errors.Equals(err, errors.ErrStop) {
				if ui.IsActive(ui.TraceLogger) {
					ui.Log(ui.TraceLogger, "trace.return", ui.A{
						"thread": c.threadID,
						"error":  err})
				}
			}

			if err != nil {
				err = errors.New(err)
			}

			return err
		}
	}

	if ui.IsActive(ui.TraceLogger) {
		ui.Log(ui.TraceLogger, "trace.end", ui.A{
			"name":   c.name,
			"thread": c.threadID})
	}

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
