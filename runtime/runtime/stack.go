package runtime

import (
	goRuntime "runtime"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

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
	size := goRuntime.Stack(buf, false)

	return data.NewList(size, nil), nil
}
