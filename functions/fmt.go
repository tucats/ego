package functions

import (
	"fmt"
	_fmt "fmt"
	"strings"

	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// Printf implements fmt.printf() and is a wrapper around the native Go function.
func Printf(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	len := 0

	str, err := Sprintf(s, args)
	if errors.Nil(err) {
		len, _ = fmt.Printf("%s", util.GetString(str))
	}

	return len, err
}

// Sprintf implements fmt.sprintf() and is a wrapper around the native Go function.
func Sprintf(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	if len(args) == 0 {
		return 0, nil
	}

	fmtString := util.GetString(args[0])

	if len(args) == 1 {
		return fmtString, nil
	}

	return fmt.Sprintf(fmtString, args[1:]...), nil
}

// Print implements fmt.Print() and is a wrapper around the native Go function.
func Print(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var b strings.Builder

	for i, v := range args {
		if i > 0 {
			b.WriteString(" ")
		}

		b.WriteString(FormatAsString(s, v))
	}

	text, e2 := fmt.Printf("%s", b.String())

	return text, errors.New(e2)
}

// Println implements fmt.Println() and is a wrapper around the native Go function.
func Println(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	var b strings.Builder

	for i, v := range args {
		if i > 0 {
			b.WriteString(" ")
		}

		b.WriteString(FormatAsString(s, v))
	}

	text, e2 := fmt.Printf("%s\n", b.String())

	return text, errors.New(e2)
}

// FormatAsString will attempt to use the String() function of the
// object type passed in, if it is a typed struct.  Otherwise, it
// just returns the Unquoted format value.
func FormatAsString(s *symbols.SymbolTable, v interface{}) string {
	if m, ok := v.(*datatypes.EgoStruct); ok {
		if f := m.GetType().Function("String"); f != nil {
			if fmt, ok := f.(func(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError)); ok {
				local := symbols.NewChildSymbolTable("local to format", s)
				_ = local.SetAlways("__this", v)

				if si, err := fmt(local, []interface{}{}); err == nil {
					if str, ok := si.(string); ok {
						return str
					}
				}
			}
		}
	}

	return util.FormatUnquoted(v)
}

func Sscanf(s *symbols.SymbolTable, args []interface{}) (interface{}, *errors.EgoError) {
	dataString := util.GetString(args[0])
	formatString := util.GetString(args[1])

	// Verify the remaining arguments are all pointers, and unwrap them.
	pointerList := make([]*interface{}, len(args)-2)

	for i, v := range args[2:] {
		if datatypes.TypeOfPointer(v).IsUndefined() {
			return nil, errors.New(errors.NotAPointer)
		}

		if content, ok := v.(*interface{}); ok {
			pointerList[i] = content
		}
	}

	// Do the scan, returning an array of values
	items, err := scanner(dataString, formatString)
	if err != nil {
		return 0, errors.New(err).Context("Sscanf()")
	}

	// Stride over the return value pointers, assigning as many
	// items as we got.
	for idx, p := range pointerList {
		if idx >= len(items) {
			break
		}

		*p = items[idx]
	}

	return len(items), nil
}

func scanner(data, format string) ([]interface{}, *errors.EgoError) {
	var err *errors.EgoError

	result := make([]interface{}, 0)

	fTokens := tokenizer.New(format)
	dTokens := tokenizer.New(data)
	d := dTokens.Tokens
	f := []string{}
	parsingVerb := false

	// Scan over the token, collapsing format verbs into a
	// single token.
	for _, token := range fTokens.Tokens {
		if parsingVerb {
			// Must only be supported format string. TODO We do not allow width
			// specifications yet.
			if !util.InList(token, "s", "t", "f", "d", "v") {
				return result, errors.New(errors.InvalidFormatVerbError)
			}

			// Add to the previous token
			f[len(f)-1] = f[len(f)-1] + token
			parsingVerb = false
		} else {
			f = append(f, token)
			if token == "%" {
				parsingVerb = true
			}
		}
	}

	parsing := true

	// Now scan over the format tokens, which now represent either
	// required tokens in the input data or format operations.
	for idx, token := range f {
		if !parsing {
			break
		}

		switch token {
		case "%v":
			var v interface{}

			_, e := _fmt.Sscanf(d[idx], "%v", &v)
			if e != nil {
				err = errors.New(e).Context("Sscanf()")
				parsing = false

				break
			}

			result = append(result, v)

		case "%s":
			result = append(result, d[idx])

		case "%t":
			v := false

			_, e := _fmt.Sscanf(d[idx], "%t", &v)
			if e != nil {
				err = errors.New(e).Context("Sscanf()")
				parsing = false

				break
			}

			result = append(result, v)

		case "%f":
			v := 0.0

			_, e := _fmt.Sscanf(d[idx], "%f", &v)
			if e != nil {
				err = errors.New(e).Context("Sscanf()")
				parsing = false

				break
			}

			result = append(result, v)

		case "%d":
			v := 0

			_, e := _fmt.Sscanf(d[idx], "%d", &v)
			if e != nil {
				err = errors.New(e).Context("Sscanf()")
				parsing = false

				break
			}

			result = append(result, v)

		default:
			if token != d[idx] {
				parsing = false

				break
			}
		}
	}

	return result, err
}
