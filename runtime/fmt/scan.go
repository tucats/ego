package fmt

import (
	nativeFormat "fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// stringScan implements the fmt.Scan() function. It accepts a source string
// and a variable number of pointer arguments. The type of each pointed-to
// value determines which format verb is used to parse the next token:
//
//   - int / int32 / int64  → %d
//   - float32 / float64    → %f
//   - string               → %s
//   - bool                 → %t
//
// All other Ego types return ErrInvalidType. The pointer arguments must be
// *any values wrapping the target value (Ego's pointer representation).
//
// Known limitation: scientific notation (e.g. 1.5e-3) is not supported for %f.
func stringScan(s *symbols.SymbolTable, args data.List) (any, error) {
	dataString := data.String(args.Get(0))
	formatString := strings.Builder{}

	// Verify the remaining arguments are all pointers, and unwrap them.
	pointerList := make([]*any, args.Len()-1)

	for i, v := range args.Elements()[1:] {
		if data.TypeOfPointer(v).IsUndefined() {
			return data.NewList(nil, errors.ErrNotAPointer), errors.ErrNotAPointer
		}

		if content, ok := v.(*any); ok {
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

// stringScanFormat implements the fmt.Sscanf() function. It accepts a source
// string, a format string, and a variable number of pointer arguments. Values
// parsed from the source string are stored through those pointers.
//
// The pointer arguments must be *any values; the format string controls which
// type is stored (see scanner for supported verbs). stringScanFormat returns
// a data.List of (count, error).
func stringScanFormat(s *symbols.SymbolTable, args data.List) (any, error) {
	dataString := data.String(args.Get(0))
	formatString := data.String(args.Get(1))

	// Verify the remaining arguments are all pointers, and unwrap them.
	pointerList := make([]*any, args.Len()-2)

	for i, v := range args.Elements()[2:] {
		if data.TypeOfPointer(v).IsUndefined() {
			return data.NewList(nil, errors.ErrNotAPointer), errors.ErrNotAPointer
		}

		if content, ok := v.(*any); ok {
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

// scanner is the core parsing engine shared by stringScan and stringScanFormat.
// It walks the format string and the data string in parallel, extracting typed
// values according to the format verbs. Supported verbs:
//
//   - %d  decimal integer
//   - %x  hexadecimal integer (case-insensitive digits)
//   - %b  binary integer
//   - %o  octal integer
//   - %f  floating-point number (decimal notation only; no leading sign)
//   - %s  whitespace-delimited string token
//   - %t  boolean ("true" or "false")
//
// Each verb may be preceded by a decimal width that limits how many characters
// are consumed (e.g. %4x reads at most 4 hex characters). Literal characters
// in the format string must match the data string exactly; a mismatch stops
// the scan without an error.
//
// Known limitation: scientific notation (e.g. 1.5e-3) is not supported for %f.
func scanner(data, format string) ([]any, error) {
	var err error

	result := make([]any, 0)
	characterSets := map[byte]string{
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
			case 'd', 'x', 'b', 'o':
				// Consume any digits for the integer value
				value := 0
				strData := ""
				fmtString := "%" + string(formatOp)

				// Allow a leading minus sign for decimal integers.
				if formatOp == 'd' && dataPos < len(data) && data[dataPos] == '-' {
					strData = "-"
					dataPos++

					if width != math.MaxInt {
						width--
					}
				}

				for width > 0 && dataPos < len(data) {
					testString := characterSets[formatOp]
					charString := string(data[dataPos])

					if !strings.Contains(testString, charString) {
						break
					}

					strData = strData + string(data[dataPos])
					dataPos++
					width--
				}

				if strData == "" || strData == "-" {
					err = errors.New(errors.ErrInvalidValue)

					break
				}

				n, err := nativeFormat.Sscanf(strData, fmtString, &value)
				if err != nil || n != 1 {
					break
				}

				result = append(result, value)

			case 's':
				// Consume any characters for the string value
				value := ""

				// If there is no width specification, skip leading spaces.
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
					// Clamp the end position so we never read past the end of data.
					end := dataPos + width
					if end > len(data) {
						end = len(data)
					}

					strData = data[dataPos:end]
					dataPos = end

					value, err = strconv.ParseFloat(strData, 64)
					if err != nil {
						break
					}
				} else {
					// Allow a leading minus sign for negative floats.
					if dataPos < len(data) && data[dataPos] == '-' {
						strData = "-"
						dataPos++
					}

					// Consume digits and the decimal point.
					for dataPos < len(data) && (data[dataPos] >= '0' && data[dataPos] <= '9' || data[dataPos] == '.') {
						strData += string(data[dataPos])
						dataPos++
					}

					if strData == "" || strData == "-" {
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
