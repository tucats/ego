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

// UseString will attempt to use the String() function of the
// object passed in, if it is a typed struct.  Otherwise, it
// just returns the Unquoted format value.
func FormatAsString(s *symbols.SymbolTable, v interface{}) string {
	if m, ok := v.(map[string]interface{}); ok {
		if f, ok := m["String"]; ok && f != nil {
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

	ptrs := make([]*interface{}, len(args)-2)

	for i, v := range args[2:] {
		if datatypes.PointerTo(v) == datatypes.UndefinedType {
			return nil, errors.New(errors.NotAPointer)
		}

		if content, ok := v.(*interface{}); ok {
			ptrs[i] = content
		}
	}

	items, err := scanner(dataString, formatString)

	if err != nil {
		return 0, errors.New(err).Context("Sscanf()")
	}

	// Stride over the return value pointers, assigning as many
	// items as we got.
	for idx, p := range ptrs {
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
	last := ""

	for _, token := range fTokens.Tokens {
		if last == "%" {
			f[len(f)-1] = "%" + token
		} else {
			f = append(f, token)
		}

		last = token
	}

	parsing := true

	for idx, token := range f {
		if !parsing {
			break
		}

		switch token {
		case "%s":
			result = append(result, d[idx])

		case "%t":
			v := false

			_, e := _fmt.Sscanf(d[idx], "%t", &v)
			if e != nil {
				err = errors.New(err).Context("Sscanf()")
				parsing = false

				break
			}

			result = append(result, v)

		case "%f":
			v := 0.0

			_, e := _fmt.Sscanf(d[idx], "%f", &v)
			if e != nil {
				err = errors.New(err).Context("Sscanf()")
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
