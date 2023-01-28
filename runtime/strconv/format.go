package strconv

import (
	"strconv"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

// Formatbool implement the strconv.Formatbool() function.
func Formatbool(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if data.Bool(args[0]) {
		return "true", nil
	}

	return "false", nil
}

// Formatfloat implement the strconv.Formatfloat() function.
func Formatfloat(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	value := data.Float64(args[0])

	fmtString := data.String(args[1])
	if !util.InList(fmtString, "f", "F", "g", "G", "b", "x", "e", "E") {
		return nil, errors.ErrInvalidFunctionArgument.In("Formatfloat").Context(fmtString)
	}

	fmt := fmtString[0]

	prec := data.Int(args[2])
	bitSize := data.Int(args[3])

	if bitSize != 32 && bitSize != 64 {
		return nil, errors.ErrInvalidBitSize.Context(bitSize).In("Formatfloat").Context(bitSize)
	}

	return strconv.FormatFloat(value, fmt, prec, bitSize), nil
}

// Formatint implement the strconv.Formatint() function.
func Formatint(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	value := data.Int64(args[0])

	base := data.Int(args[1])
	if base < 2 || base > 36 {
		return nil, errors.ErrInvalidFunctionArgument.In("Formatfloat").Context(base)
	}

	return strconv.FormatInt(value, base), nil
}
