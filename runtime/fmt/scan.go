package fmt

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// stringScanFormat implements the fmt.Scan() function. This accepts a string
// containing arbitrary data and a variable list of addresses to arbitrary objects,
// which will receive the input values from the data string that are scanned.
//
// This works by evaluating the arguments, and creating a suitable format string
// which is then passed to the Sscanf runtime function.
func stringScan(s *symbols.SymbolTable, args data.List) (interface{}, error) {
	dataString := data.String(args.Get(0))
	formatString := strings.Builder{}

	// Verify the remaining arguments are all pointers, and unwrap them.
	pointerList := make([]*interface{}, args.Len()-1)

	for i, v := range args.Elements()[1:] {
		if data.TypeOfPointer(v).IsUndefined() {
			return data.NewList(nil, errors.ErrNotAPointer), errors.ErrNotAPointer
		}

		if content, ok := v.(*interface{}); ok {
			switch data.TypeOf(*content) {
			case data.IntType, data.Int32Type, data.Int64Type:
				formatString.WriteString("%d")

			case data.Float32Type, data.Float64Type:
				formatString.WriteString("%f")

			case data.StringType:
				formatString.WriteString("%s")

			case data.BoolType:
				formatString.WriteString("%t")

			default:
				return data.NewList(nil, errors.ErrInvalidType), errors.ErrInvalidType
			}

			pointerList[i] = content
		}
	}

	// Do the scan, returning an array of values
	items, err := scanner(dataString, formatString.String())
	if err != nil {
		err = errors.New(err).In("Scan")

		return data.NewList(0, err), err
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

// scanner is the core of the Sscanf() function. It accepts a data string and
// a format string, and returns an array of values that were scanned from the
// data string. The format string is a series of format specifiers, each of
// which is introduced by a % character. The format specifiers are:
//
// %d - integer value
// %x - hexadecimal integer value
// %b - binary integer value
// %o - octal integer value
// %f - floating point value
// %s - string value
// %t - boolean value
//
// Each format specifier can be preceded by a width specifier, which is a
// decimal integer value that indicates the maximum number of characters
// to consume for that format specifier. If no width specifier is present,
// the width is unlimited.
//
// The format string can also contain literal characters, which are matched
// against the data string. If a literal character is encountered that does
// not match the data string, the scan is terminated.
//
// The return value is an array of values that were scanned from the data
// string. If an error occurs, the array is empty and the error is returned.
func scanner(data, format string) ([]interface{}, error) {
	var err error

	result := make([]interface{}, 0)
	charsets := map[byte]string{
		'd': "0123456789",
		'x': "0123456789abcdefABCDEF",
		'b': "01",
		'o': "01234567",
	}
	formatPos := 0
	dataPos := 0

	for formatPos < len(format) {
		// If this is a blank in the format string, no work yet.
		if unicode.IsSpace(rune(format[formatPos])) {
			formatPos++

			continue
		}

		// If the format string has a %, then we have a format
		// specifier. If not, we have a literal string.
		if format[formatPos] == '%' {
			formatPos++
			if formatPos >= len(format) {
				return result, errors.New(errors.ErrInvalidFormatVerb)
			}

			// Skip any leading blanks. If we hit the end of the data, we're done.
			if dataPos = skipSpaces(data, dataPos); dataPos >= len(data) {
				break
			}

			// Consume any digits for the width specifier
			width := 0
			widthSpecified := false

			for format[formatPos] >= '0' && format[formatPos] <= '9' {
				width = width*10 + int(format[formatPos]-'0')
				formatPos++
			}

			if width == 0 {
				width = math.MaxInt
			} else {
				widthSpecified = true
			}

			// Based on the next character, process the format
			// specifier.
			formatOp := format[formatPos]
			formatPos++

			switch formatOp {
			case 'T':
				dataPos = skipSpaces(data, dataPos)
				endPos := findSpace(data, dataPos)
				value := data[dataPos:endPos]

				// Use the compiler to parse the string into a Type value
				t, err := compiler.CompileTypeSpec(value, nil)
				if err != nil {
					break
				}

				result = append(result, t)
				dataPos = endPos

			case 'd', 'x', 'b', 'o':
				// Consume any digits for the integer value
				value := 0
				strData := ""
				fmtString := "%" + string(formatOp)

				for width > 0 && dataPos < len(data) {
					testString := charsets[formatOp]
					charString := string(data[dataPos])

					if !strings.Contains(testString, charString) {
						break
					}

					strData = strData + string(data[dataPos])
					dataPos++
					width--
				}

				if strData == "" {
					err = errors.New(errors.ErrInvalidValue)

					break
				}

				n, err := fmt.Sscanf(strData, fmtString, &value)
				if err != nil || n != 1 {
					break
				}

				result = append(result, value)

			case 's':
				// Consume any characters for the string value
				value := ""

				// If there is no width specificatin, skip leading spaces.
				if width == math.MaxInt {
					for dataPos < len(data) && unicode.IsSpace(rune(data[dataPos])) {
						dataPos++
					}
				}

				// Scoop up characters until we hit a space or the requested width.
				spaceIndex := findSpace(data, dataPos)
				if widthSpecified {
					if spaceIndex < dataPos+width {
						spaceIndex = dataPos + width
					}

					if spaceIndex > dataPos+width {
						spaceIndex = dataPos + width
					}
				}

				value = data[dataPos:spaceIndex]
				dataPos = spaceIndex

				result = append(result, value)

			case 'f':
				// Consume any digits for the floating point value
				value := 0.0
				strData := ""

				if widthSpecified {
					strData = data[dataPos : dataPos+width]
					dataPos += width

					value, err = strconv.ParseFloat(strData, 64)
					if err != nil {
						break
					}
				} else {
					// Consume any characters that are valid floating point
					// digits.
					for dataPos < len(data) && (data[dataPos] >= '0' && data[dataPos] <= '9' || data[dataPos] == '.') {
						strData += string(data[dataPos])
						dataPos++
					}

					if strData == "" {
						err = errors.New(errors.ErrInvalidValue)

						break
					}

					value, err = strconv.ParseFloat(strData, 64)
					if err != nil {
						break
					}
				}

				result = append(result, value)

			case 't':
				value := false
				if strings.HasPrefix(data[dataPos:], "true") {
					value = true
					dataPos += 4
				} else if strings.HasPrefix(data[dataPos:], "false") {
					dataPos += 5
				} else {
					return result, errors.New(errors.ErrInvalidBooleanValue)
				}

				result = append(result, value)

			default:
				return result, errors.New(errors.ErrInvalidFormatVerb)
			}
		} else {
			// This is a literal string. Consume characters that match between
			// the format string and the data string
			count := 0
			literal := ""

			// Advance the format position until we find a non-blank character
			formatPos = skipSpaces(format, formatPos)

			// Advance the data position until we find a non-blank character
			dataPos = skipSpaces(data, dataPos)

			// Now check the characters in sequence.
			for formatPos < len(format) && dataPos < len(data) {
				ch1 := string(data[dataPos])
				ch2 := string(format[formatPos])

				if ch1 != ch2 {
					break
				}

				if format[formatPos] == '%' {
					break
				}

				literal += string(data[dataPos])

				count++
				formatPos++
				dataPos++
			}

			if count == 0 {
				break
			}
		}
	}

	return result, err
}

// skipSpaces is a helper function that skips over any spaces in the data string.
func skipSpaces(data string, pos int) int {
	for pos < len(data) && unicode.IsSpace(rune(data[pos])) {
		pos++
	}

	return pos
}

// findSpace is a helper function that finds the next space in the data string.
// Note that spaces that are enclosed in quotes, braces, brackets, or parentheses
// do not count as spaces for this purpose.
func findSpace(data string, pos int) int {
	doubleQuote := false
	singleQuote := false
	braceLevel := 0
	parenLevel := 0
	angleLevel := 0

	for pos < len(data) {
		if data[pos] == '<' {
			angleLevel++
		} else if data[pos] == '>' {
			angleLevel--
		} else if data[pos] == '(' {
			parenLevel++
		} else if data[pos] == ')' {
			parenLevel--
		} else if data[pos] == '{' {
			braceLevel++
		} else if data[pos] == '}' {
			braceLevel--
		} else if data[pos] == '"' && !singleQuote {
			doubleQuote = !doubleQuote
		} else if data[pos] == '\'' && !doubleQuote {
			singleQuote = !singleQuote
		} else if !doubleQuote && !singleQuote && braceLevel == 0 && parenLevel == 0 && angleLevel == 0 && unicode.IsSpace(rune(data[pos])) {
			break
		}

		pos++
	}

	return pos
}
