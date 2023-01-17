package functions

import (
	"strconv"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/util"
)

func StrConvAtoi(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	str := data.String(args[0])

	if v, err := strconv.Atoi(str); err != nil {
		return nil, errors.NewError(err).Context("Atoi")
	} else {
		return v, nil
	}
}

func StrConvItoa(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	value := data.Int(args[0])

	return strconv.Itoa(value), nil
}

func StrConvFormatBool(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if data.Bool(args[0]) {
		return "true", nil
	}

	return "false", nil
}

func StrConvFormatFloat(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
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

func StrConvFormatInt(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	value := data.Int64(args[0])

	base := data.Int(args[1])
	if base < 2 || base > 36 {
		return nil, errors.ErrInvalidFunctionArgument.In("Formatfloat").Context(base)
	}

	return strconv.FormatInt(value, base), nil
}

func StrConvQuote(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	value := data.String(args[0])

	return strconv.Quote(value), nil
}

func StrConvUnquote(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	value := data.String(args[0])

	if v, err := strconv.Unquote(value); err != nil {
		return nil, errors.NewError(err).In("Unquote")
	} else {
		return v, nil
	}
}
