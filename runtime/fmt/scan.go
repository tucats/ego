package fmt

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
)

// stringScanFormat implements the fmt.Sscanf() function. This accepts a string
// containing arbitrary data, a format string that guides the scanner in how to
// interpret the string, and a variable list of addresses to arbitrary objects,
// which will receive the input values from the data string that are scanned.
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
		return data.NewList(0, err), errors.New(err).In("Sscanf")
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
	dataIndex := -1

	for idx := 0; idx < len(f); idx++ {
		if !parsing {
			break
		}

		var data tokenizer.Token

		dataIndex = dataIndex + 1
		token := f[idx]

		if dataIndex >= len(d) {
			data = tokenizer.NewToken(tokenizer.StringTokenClass, "")
		} else {
			data = d[dataIndex]
		}

		if token[:1] == "%" {
			switch token[len(token)-1:] {
			case "v":
				var v interface{}

				_, e := fmt.Sscanf(data.Spelling(), token, &v)
				if e != nil {
					err = errors.New(e).In("Sscanf")
					parsing = false

					break
				}

				result = append(result, v)

			case "s":
				v := ""
				l := 0

				// If the token string is longer than two characters, extract
				// the integer length of the string to scan from between the
				// "%" and "s" characters.
				if len(token) > 2 {
					lenStr := token[1 : len(token)-1]

					l, err = strconv.Atoi(lenStr)
					if err != nil {
						err = errors.New(err).In("Sscanf")
						parsing = false

						break
					}
				}

				dataStr := strings.TrimSpace(data.Spelling())
				if l == 0 {
					v = dataStr
				} else {
					if l > len(dataStr) {
						l = len(dataStr)
					}

					v = dataStr[:l]
					if l < len(dataStr) {
						d[dataIndex] = tokenizer.NewToken(tokenizer.StringTokenClass, dataStr[l:])
						dataIndex = dataIndex - 1
					}
				}

				result = append(result, v)

			case "t":
				v := false
				dataStr := strings.TrimSpace(data.Spelling())

				if strings.HasPrefix(dataStr, "true") {
					v = true
					dataStr = dataStr[4:]
				} else if strings.HasPrefix(dataStr, "false") {
					v = false
					dataStr = dataStr[5:]
				} else {
					err = errors.New(errors.ErrInvalidBooleanValue).Context(data.Spelling()).In("Sscanf")
					parsing = false

					break
				}

				result = append(result, v)

				// Was there data in the token after the boolean string value? If so,
				// put it back as the current token value and back up the data token
				// pointer.
				if len(dataStr) > 0 {
					d[dataIndex] = tokenizer.NewToken(tokenizer.StringTokenClass, dataStr)
					dataIndex = dataIndex - 1
				}

			case "f":
				v := 0.0
				l := 0

				if len(token) > 2 {
					lenStr := token[1 : len(token)-1]

					l, err = strconv.Atoi(lenStr)
					if err != nil {
						err = errors.New(err).In("Sscanf")
						parsing = false
					}
				}

				dataStr := data.Spelling()
				if l > 0 {
					if l > len(dataStr) {
						d[dataIndex] = tokenizer.NewToken(tokenizer.StringTokenClass, dataStr[l:])
						dataIndex = dataIndex - 1
					}

					dataStr = dataStr[:l]
				}

				_, e := fmt.Sscanf(dataStr, token, &v)
				if e != nil {
					err = errors.New(e).In("Sscanf")
					parsing = false

					break
				}

				result = append(result, v)

			case "d", "b", "x", "o":
				v := 0
				l := 0

				if len(token) > 2 {
					lenStr := token[1 : len(token)-1]

					l, err = strconv.Atoi(lenStr)
					if err != nil {
						err = errors.New(err).In("Sscanf")
						parsing = false
					}
				}

				dataStr := data.Spelling()
				if l > 0 {
					if l > len(dataStr) {
						d[dataIndex] = tokenizer.NewToken(tokenizer.StringTokenClass, dataStr[l:])
						dataIndex = dataIndex - 1
					}

					dataStr = dataStr[:l]
				}

				_, e := fmt.Sscanf(dataStr, token, &v)
				if e != nil {
					err = errors.New(e).In("Sscanf")
					parsing = false

					break
				}

				result = append(result, v)
			}
		} else {
			if token != data.Spelling() {
				parsing = false
			}
		}
	}

	return result, err
}
