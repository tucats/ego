package bytecode

import (
	"strconv"

	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

const (
	// Discards the catch set, which means all errors are caught.
	AllErrorsCatchSet = 0

	// Set of errors that an ?optional is permitted to ignore.
	OptionalCatchSet = 1
)

var catchSets = [][]*errors.EgoError{
	// OptionalCatchSet
	{
		errors.New(errors.UnknownMemberError),
		errors.New(errors.InvalidTypeError),
		errors.New(errors.NilPointerReferenceError),
		errors.New(errors.DivisionByZeroError),
	},
}

// TryImpl instruction processor.
func TryImpl(c *Context, i interface{}) *errors.EgoError {
	try := TryInfo{
		addr:    util.GetInt(i),
		catches: make([]*errors.EgoError, 0),
	}
	c.try = append(c.try, try)

	return nil
}

// WillCatch instruction. This lets the code specify which errors
// are permitted to be caught; if the list is empty then all errors
// are caught.
func willCatch(c *Context, i interface{}) *errors.EgoError {
	if len(c.try) == 0 {
		return c.newError(errors.TryCatchMismatchError)
	}

	try := c.try[len(c.try)-1]
	if try.catches == nil {
		try.catches = make([]*errors.EgoError, 0)
	}

	switch i := i.(type) {
	case int:
		if i > len(catchSets) {
			return c.newError(errors.InternalCompilerError).Context("invalid catch set " + strconv.Itoa(i))
		}

		// Zero has a special meaning of "catch everything"
		if i == AllErrorsCatchSet {
			try.catches = make([]*errors.EgoError, 0)
		} else {
			try.catches = append(try.catches, catchSets[i-1]...)
		}

	case *errors.EgoError:
		try.catches = append(try.catches, i)

	case error:
		try.catches = append(try.catches, errors.New(i))

	case string:
		try.catches = append(try.catches, errors.NewMessage(i))

	default:
		return c.newError(errors.InvalidTypeError)
	}

	c.try[len(c.try)-1] = try

	return nil
}

// TryPopImpl instruction processor.
func TryPopImpl(c *Context, i interface{}) *errors.EgoError {
	if len(c.try) == 0 {
		return c.newError(errors.TryCatchMismatchError)
	}

	if len(c.try) == 1 {
		c.try = make([]TryInfo, 0)
	} else {
		c.try = c.try[:len(c.try)-1]
	}

	_ = c.symbols.Delete("_error", true)

	return nil
}
