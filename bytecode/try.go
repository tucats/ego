package bytecode

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
)

// Constant that describes using no catch set, which means all errors
// are caught.
const allErrorsCatchSet = 0

// Constant that describes using the set of errors that an ?optional
// is permitted to ignore.
const OptionalCatchSet = 1

var catchSets = [][]error{
	// OptionalCatchSet
	{
		errors.ErrUnknownMember,
		errors.ErrInvalidType,
		errors.ErrNilPointerReference,
		errors.ErrDivisionByZero,
		errors.ErrArrayIndex,
		errors.ErrTypeMismatch,
		errors.ErrInvalidValue,
		errors.ErrInvalidInteger,
		errors.ErrInvalidFloatValue,
	},
}

// tryByteCode instruction processor.
func tryByteCode(c *Context, i interface{}) error {
	addr, err := data.Int(i)
	if err != nil {
		return c.runtimeError(err)
	}

	try := tryInfo{
		addr:    addr,
		catches: make([]error, 0),
	}
	c.tryStack = append(c.tryStack, try)

	return nil
}

// WillCatch instruction. This lets the code specify which errors
// are permitted to be caught; if the list is empty then all errors
// are caught.
func willCatchByteCode(c *Context, i interface{}) error {
	if len(c.tryStack) == 0 {
		return c.runtimeError(errors.ErrTryCatchMismatch)
	}

	try := c.tryStack[len(c.tryStack)-1]
	if try.catches == nil {
		try.catches = make([]error, 0)
	}

	switch i := i.(type) {
	case int:
		if i > len(catchSets) {
			return c.runtimeError(errors.ErrInternalCompiler).Context(i18n.E("invalid.catch.set",
				map[string]interface{}{"index": i}))
		}

		// Zero has a special meaning of "catch everything"
		if i == allErrorsCatchSet {
			try.catches = make([]error, 0)
		} else {
			try.catches = append(try.catches, catchSets[i-1]...)
		}

	case *errors.Error:
		try.catches = append(try.catches, i)

	case error:
		try.catches = append(try.catches, errors.New(i))

	case string:
		try.catches = append(try.catches, errors.Message(i))

	default:
		return c.runtimeError(errors.ErrInvalidType).Context(data.TypeOf(i).String())
	}

	c.tryStack[len(c.tryStack)-1] = try

	return nil
}

// tryPopByteCode instruction processor.
func tryPopByteCode(c *Context, i interface{}) error {
	if len(c.tryStack) == 0 {
		return c.runtimeError(errors.ErrTryCatchMismatch)
	}

	if len(c.tryStack) == 1 {
		c.tryStack = make([]tryInfo, 0)
	} else {
		c.tryStack = c.tryStack[:len(c.tryStack)-1]
	}

	_ = c.symbols.Delete(defs.ErrorVariable, true)

	return nil
}

// Flush the try stack, so any subsequent errors will be signaled through instead
// of caught.
func tryFlushByteCode(c *Context, i interface{}) error {
	c.tryStack = make([]tryInfo, 0)

	return nil
}
