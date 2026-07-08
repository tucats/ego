package bytecode

import (
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/errors"
)

// runtimeError is a helper function used to generate a new runtimeError based
// on the runtime context. The current module name and line number
// from the context are stored in the new runtimeError object, along with
// the message and context.
func (c *Context) runtimeError(err error, context ...any) *errors.Error {
	if err == nil {
		return nil
	}

	var r *errors.Error

	// If this is already an error, make a clone of it and add the module and location
	// info and return it.
	if e, ok := err.(*errors.Error); ok {
		e = e.Clone()

		if !e.HasIn() {
			if c.module != "" {
				e = e.In(c.module)
			} else if c.name != "" {
				e = e.In(c.name)
			}
		}

		if !e.HasAt() {
			e = e.At(c.GetLine(), 0)
		}

		r = e
	} else {
		// Construct a new error with the current module name and line number.
		r = errors.New(err).In(c.name).At(c.GetLine(), 0)
	}

	if len(context) > 0 {
		r = r.Context(context[0])
	}

	return r
}

// Implement the Signal bytecode, which generates an arbitrary error return,
// using the instruction opcode.
func signalByteCode(c *Context, i any) error {
	if i == nil {
		if v, err := c.Pop(); err != nil {
			return err
		} else {
			i = v
		}
	}

	if e, ok := i.(*errors.Error); ok {
		return c.runtimeError(e)
	}

	if e, ok := i.(error); ok {
		return c.runtimeError(errors.New(e))
	}

	return c.runtimeError(errors.Message(data.String(i)))
}

// Implement the Throw bytecode, used by the "throw" statement extension
// (see compileThrow). It pops a value from the stack that is expected to be
// an error. Unlike Signal (used by the @error test directive, which always
// raises whatever is on the stack, converting non-error values into an
// error message), Throw treats a nil -- or zero-value -- error as "nothing
// to throw" and lets execution continue normally. This collapses Go's
// "if err != nil { return err }" idiom into a single statement: "throw err"
// only raises a trappable, catchable error (exactly like a genuine runtime
// error) when err actually represents an error condition.
//
// data.IsNil is used rather than a plain "v == nil" comparison because
// Ego's zero-value error (e.g. the receiver of "var e error" with no
// initializer) is a non-nil *errors.Error with an empty inner error, which
// is nonetheless considered "no error" throughout the language (see
// docs/LANGUAGE.md's error section and BUG-65 in docs/ISSUES.md).
func throwByteCode(c *Context, i any) error {
	v, err := c.Pop()
	if err != nil {
		return err
	}

	if isStackMarker(v) {
		return c.runtimeError(errors.ErrFunctionReturnedVoid)
	}

	if data.IsNil(v) {
		return nil
	}

	if e, ok := v.(*errors.Error); ok {
		return c.runtimeError(e)
	}

	if e, ok := v.(error); ok {
		return c.runtimeError(errors.New(e))
	}

	return c.runtimeError(errors.ErrArgumentType).Context(data.TypeOf(v).String())
}
