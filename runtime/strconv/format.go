package strconv

import (
	"strconv"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// doFormatbool implement the strconv.doFormatbool() function.
func doFormatbool(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	if data.Bool(args.Get(0)) {
		return "true", nil
	}

	return "false", nil
}

// doFormatfloat implement the strconv.doFormatfloat() function.
func doFormatfloat(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	value := data.Float64(args.Get(0))

	fmtString := data.String(args.Get(1))
	if !util.InList(fmtString, "f", "F", "g", "G", "b", "x", "e", "E") {
		return nil, errors.ErrInvalidFunctionArgument.In("Formatfloat").Context(fmtString)
	}

	fmt := fmtString[0]

	prec := data.Int(args.Get(2))
	bitSize := data.Int(args.Get(3))

	if bitSize != 32 && bitSize != 64 {
		return nil, errors.ErrInvalidBitSize.Context(bitSize).In("Formatfloat").Context(bitSize)
	}

	return strconv.FormatFloat(value, fmt, prec, bitSize), nil
}

// doFormatint implement the strconv.doFormatint() function.
func doFormatint(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	value := data.Int64(args.Get(0))

	base := data.Int(args.Get(1))
	if base < 2 || base > 36 {
		return nil, errors.ErrInvalidFunctionArgument.In("Formatfloat").Context(base)
	}

	return strconv.FormatInt(value, base), nil
}
