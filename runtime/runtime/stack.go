package runtime

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// Implement the runtime.Stack() function. Accepts a []byte argument
// and fills it with a text representation of the Go call frame at
// this point in the process.
func stack(s *symbols.SymbolTable, args data.List) (any, error) {
	// Get the []byte buffer provided by the caller. This will be
	// an Ego array...
	v := args.Get(0)
	argErr := errors.ErrArgumentType.In("runtime.Stack")

	b, ok := v.(*data.Array)
	if !ok {
		return data.NewList(nil, argErr), argErr
	}

	buf := b.GetBytes()

	// We should find the runtime context as the secret last value
	// in the argument list...
	var c *bytecode.Context

	if vc, ok := args.Get(args.Len() - 1).(*bytecode.Context); ok {
		c = vc
	}

	text := c.FormatFrames(bytecode.IncludeSymbolTableNames)

	size := len(text)
	if size > len(buf) {
		size = len(buf)
	}

	// Copy the text bytes of the result (as much as will fit) into the
	// destination buffer
	for i := 0; i < size; i++ {
		buf[i] = text[i]
	}

	return data.NewList(size, nil), nil
}

func frames(s *symbols.SymbolTable, args data.List) (any, error) {
	// Get the initial  count from the caller.
	count, err := data.Int(args.Get(0))
	if err != nil {
		return data.NewList(nil, errors.New(err)), errors.New(err)
	}

	// We should find the runtime context as the secret last value
	// in the argument list...
	var c *bytecode.Context

	if vc, ok := args.Get(args.Len() - 1).(*bytecode.Context); ok {
		c = vc
	}

	if c == nil {
		return data.NewList(nil, errors.ErrInternalCompiler.Context("no context value in arglist")), nil
	}

	result := data.NewArray(FrameType, 0)

	for i := range count {
		moduleName, sourceLineNumber, symbolTableName := c.GetFrame(i + 1)
		if moduleName == "" && sourceLineNumber == 0 {
			break
		}

		frame := data.NewStructOfTypeFromMap(FrameType, map[string]any{
			"Module": moduleName,
			"Table":  symbolTableName,
			"Line":   sourceLineNumber,
		})

		result.Append(frame)
	}

	return data.NewList(result, nil), nil
}
