package fmt

import (
	"fmt"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// stringScanFormat implements the fmt.stringScanFormat() function. This accepts a string containing
// arbitrary data, a format string that guides the scanner in how to interpret
// the string, and a variable list of addresses to arbitrary objects, which
// will receive the input values from the data string that are scanned.
func stringScanFormat(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	dataString := data.String(args.Get(0))
	formatString := data.String(args.Get(1))

	// Verify the remaining arguments are all pointers, and unwrap them.
	pointerList := make([]*interface{}, args.Len()-2)

	for i, v := range args.Elements()[2:] {
		if data.TypeOfPointer(v).IsUndefined() {
			return data.NewList(nil, errors.ErrNotAPointer), errors.ErrNotAPointer
		}

		if content, ok := v.(*interface{}); ok {
			pointerList[i] = content
		}
	}

	// Do the scan, returning an array of values
	items, err := scanner(dataString, formatString)
	if err != nil {
		return data.NewList(0, err), errors.NewError(err).In("Sscanf")
	}

	// Stride over the return value pointers, assigning as many
	// items as we got.
	for idx, p := range pointerList {
		if idx >= len(items) {
			break
		}

		*p = items[idx]
	}

	return data.NewList(len(items), nil), nil
}

func scanner(data, format string) ([]interface{}, error) {
	var (
		err         error
		parsingVerb bool
		result      = make([]interface{}, 0)
		fTokens     = tokenizer.New(format, false)
		dTokens     = tokenizer.New(data, false)
		f           = []string{}
	)

	d := dTokens.Tokens

	// Scan over the token, collapsing format verbs into a
	// single token.
	for _, token := range fTokens.Tokens {
		if parsingVerb {
			// Add to the previous token
			f[len(f)-1] = f[len(f)-1] + token.Spelling()

			// Are we at the end of a supported format string?
			if util.InList(token.Spelling(), "b", "x", "o", "s", "t", "f", "d", "v") {
				parsingVerb = false
			}
		} else {
			f = append(f, token.Spelling())
			if token == tokenizer.ModuloToken {
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

		if token[:1] == "%" {
			switch token[len(token)-1:] {
			case "v":
				var v interface{}

				_, e := fmt.Sscanf(d[idx].Spelling(), token, &v)
				if e != nil {
					err = errors.NewError(e).In("Sscanf")
					parsing = false

					break
				}

				result = append(result, v)

			case "s":
				v := ""

				_, e := fmt.Sscanf(d[idx].Spelling(), token, &v)
				if e != nil {
					err = errors.NewError(e).In("Sscanf")
					parsing = false

					break
				}

				result = append(result, v)

			case "t":
				v := false

				_, e := fmt.Sscanf(d[idx].Spelling(), token, &v)
				if e != nil {
					err = errors.NewError(e).In("Sscanf")
					parsing = false

					break
				}

				result = append(result, v)

			case "f":
				v := 0.0

				_, e := fmt.Sscanf(d[idx].Spelling(), token, &v)
				if e != nil {
					err = errors.NewError(e).In("Sscanf")
					parsing = false

					break
				}

				result = append(result, v)

			case "d", "b", "x", "o":
				v := 0

				_, e := fmt.Sscanf(d[idx].Spelling(), token, &v)
				if e != nil {
					err = errors.NewError(e).In("Sscanf")
					parsing = false

					break
				}

				result = append(result, v)
			}
		} else {
			if token != d[idx].Spelling() {
				parsing = false
			}
		}
	}

	return result, err
}
