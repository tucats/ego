package bytecode

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// argCheckByteCode instruction processor verifies that there are enough items
// on the stack to satisfy the function's argument list. The operand is the
// number of values that must be available. Alternatively, the operand can be
// an array of objects, which are the minimum count, maximum count, and
// function name.
func argCheckByteCode(c *Context, i any) error {
	var (
		err         error
		minArgCount int
		maxArgCount int
		name        = "function call"
	)

	// The operand can be an array of values, or a single integer.
	switch operand := i.(type) {
	case []any:
		// ArgCheck is normally stored as an array interface.
		if len(operand) < 2 || len(operand) > 3 {
			return c.runtimeError(errors.ErrArgumentTypeCheck)
		}

		if minArgCount, err = data.Int(operand[0]); err != nil {
			return c.runtimeError(err)
		}

		if maxArgCount, err = data.Int(operand[1]); err != nil {
			return c.runtimeError(err)
		}

		if len(operand) == 3 {
			v := operand[2]
			if s, ok := v.(string); ok {
				name = s
			} else if t, ok := v.(tokenizer.Token); ok {
				name = t.Spelling()
			} else {
				name = data.String(v)
			}

			c.module = name
		}

	case int:
		if operand >= 0 {
			minArgCount = operand
			maxArgCount = operand
		} else {
			minArgCount = 0
			maxArgCount = -operand
		}

	case []int:
		if len(operand) != 2 {
			return c.runtimeError(errors.ErrArgumentTypeCheck)
		}

		minArgCount = operand[0]
		maxArgCount = operand[1]

	default:
		return c.runtimeError(errors.ErrArgumentTypeCheck)
	}

	// Since this function could call itself recursively, ensure the current frame's
	// symbol table has an entry for this function so recursive calls can resolve it.
	// GetAnyScope traverses all parent tables (ignoring scope boundaries), but Get
	// respects boundaries and can skip intermediate frames. By storing the function
	// in every invocation's local table we guarantee it is always found locally.
	fName := c.module
	if f, found := c.symbols.GetAnyScope(fName); fName == name && found {
		var fd data.Function

		switch fv := f.(type) {
		case *ByteCode:
			if fv.declaration != nil {
				fd = data.Function{
					Declaration: fv.declaration,
					Value:       fv,
				}
			}
		case data.Function:
			fd = fv
		}

		if fd.Value != nil {
			err = c.symbols.Create(fName)
			if err == nil {
				err = c.symbols.Set(fName, fd)
			}
		}

		if err != nil {
			return c.runtimeError(err)
		}
	}

	// Check the arguments passed to us...
	args, found := c.get(defs.ArgumentListVariable)
	if !found {
		return c.runtimeError(errors.ErrArgumentTypeCheck)
	}

	// Do the actual compare. Note that if we ended up with a negative
	// max, that means variable argument list size, and we just assume
	// what we found in the max...
	if array, ok := args.(*data.Array); ok {
		if maxArgCount < 0 {
			maxArgCount = array.Len()
		}

		if array.Len() < minArgCount || array.Len() > maxArgCount {
			return c.runtimeError(errors.ErrArgumentCount).In(name)
		}

		return nil
	}

	return c.runtimeError(errors.ErrArgumentTypeCheck)
}
