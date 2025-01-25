package i18n

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func HandleSubstitutionMap(text string, valueMap map[string]interface{}) string {
	if len(valueMap) == 0 {
		return text
	}

	// Before we get cranking, fix any escaped newlines.
	text = strings.ReplaceAll(text, "\\n", "\n")

	return handleSubstitutionMap(text, valueMap)
}

func handleSubstitutionMap(text string, subs map[string]interface{}) string {
	// Split the string into parts based on locating the placeholder tokens surrounded by "((" and "}}"
	if !strings.Contains(text, "{{") {
		return text
	}

	parts := splitOutFormats(text)

	for idx, part := range parts {
		if !strings.HasPrefix(part, "{{") || !strings.HasSuffix(part, "}}") {
			continue
		}

		parts[idx] = handleFormat(part, subs)
	}

	text = strings.Join(parts, "")

	return text
}

func splitOutFormats(text string) []string {
	parts := make([]string, 0)
	segments := strings.Split(text, "{{")

	for _, segment := range segments {
		if segment == "" {
			continue
		}

		if !strings.Contains(segment, "}}") {
			parts = append(parts, segment)

			continue
		}

		subparts := strings.SplitN(segment, "}}", 2)
		parts = append(parts, "{{"+subparts[0]+"}}")

		if len(subparts[1]) > 0 {
			parts = append(parts, subparts[1])
		}
	}

	return parts
}

func handleFormat(text string, subs map[string]interface{}) string {
	var (
		result string
		err    error
	)

	if !strings.HasPrefix(text, "{{") || !strings.HasSuffix(text, "}}") {
		return text
	}

	key := strings.TrimSuffix(strings.TrimPrefix(text, "{{"), "}}")

	key, format, found := strings.Cut(key, "|")
	if !found || format == "" {
		format = "%v"
	}

	value, ok := subs[key]
	if !ok {
		value = "!" + key + "!"
		format = "%s"
	}

	// Check for special cases in the format string
	formatParts := barUnescape(strings.Split(barEscape(format), "|"))

	label := ""
	format = "%v"

	for _, part := range formatParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		switch {
		case strings.HasPrefix(part, "size "):
			sizeParm := strings.TrimSpace(part[len("size "):])
			if size, err := strconv.Atoi(sizeParm); err == nil && size > 4 {
				if format == "" {
					text = value.(string)
				} else {
					text = fmt.Sprintf(format, value)
				}

				if len(text) > size {
					value = text[:size-3] + "..."
				}
			} else {
				value = "!Invalid size: " + sizeParm + "!"
			}

			format = ""

		case part == "lines":
			value = makeLines(value, format)
			format = ""

		case strings.HasPrefix(part, "%"):
			format = part

		case strings.HasPrefix(part, "label "):
			if !isZeroValue(value) {
				label = strings.TrimSpace(part[len("label "):])

				if unquoted, err := strconv.Unquote(label); err == nil {
					label = unquoted
				}
			} else {
				label = ""
				format = "%s"
				value = ""
			}

		case strings.HasPrefix(part, "pad "):
			pad := strings.TrimSpace(part[len("pad "):])
			if strings.HasPrefix(pad, "\"") {
				pad, _ = strconv.Unquote(pad)
			}

			var count int

			switch v := value.(type) {
			case int:
				count = v

			case float64:
				count = int(math.Round(v))

			case string:
				count, err = strconv.Atoi(v)
				if err != nil || count < 0 {
					return "!Invalid pad count: " + part + "!"
				}

			default:
				return "!Invalid pad type: " + part + "!"
			}

			if err != nil || count < 0 {
				return "!Invalid pad count: " + part + "!"
			}

			value = strings.Repeat(pad, count)

		case strings.HasPrefix(part, "left "):
			pad := strings.TrimSpace(part[len("left "):])

			count, err := strconv.Atoi(pad)
			if err != nil || count < 0 {
				return "!Invalid left count: " + part + "!"
			}

			var text string

			if format == "" {
				text = value.(string)
			} else {
				text = fmt.Sprintf(format, value)
			}

			for len(text) < count {
				text = text + " "
			}
			value = text

		case strings.HasPrefix(part, "right "):
			pad := strings.TrimSpace(part[len("left "):])

			count, err := strconv.Atoi(pad)
			if err != nil || count < 0 {
				return "!Invalid left count: " + part + "!"
			}

			var text string

			if format == "" {
				text = value.(string)
			} else {
				text = fmt.Sprintf(format, value)
			}

			for len(text) < count {
				text = " " + text
			}

			value = text

		case strings.HasPrefix(part, "center "):
			pad := strings.TrimSpace(part[len("center "):])

			count, err := strconv.Atoi(pad)
			if err != nil || count < 0 {
				return "!Invalid center count: " + part + "!"
			}

			var text string

			if format == "" {
				text = value.(string)
			} else {
				text = fmt.Sprintf(format, value)
			}

			isLeft := false

			for len(text) < count {
				if isLeft {
					text = " " + text
				} else {
					text = text + " "
				}

				isLeft = !isLeft
			}

			value = text
			format = ""

		case strings.HasPrefix(part, "format "):
			format = strings.TrimSpace(part[len("format "):])

		case strings.HasPrefix(part, "empty"):
			replacement := strings.TrimSpace(part[len("empty"):])

			if unquoted, err := strconv.Unquote(replacement); err == nil {
				replacement = unquoted
			}

			if isZeroValue(value) {
				value = replacement
				format = "%s"
			}

		case strings.HasPrefix(part, "list"):
			value = makeList(value, format)
			format = ""

		case strings.HasPrefix(part, "nonempty"):
			replacement := strings.TrimSpace(part[len("nonempty"):])

			if unquoted, err := strconv.Unquote(replacement); err == nil {
				replacement = unquoted
			}

			if !isZeroValue(value) {
				value = replacement
				format = "%s"
			}

		default:
			return "!Invalid format: " + part + "!"
		}
	}

	if format == "" {
		result = fmt.Sprintf("%s%s", label, value)
	} else {
		result = fmt.Sprintf("%s"+format, label, value)
	}

	return result
}

// Search a string value for a "|" in quotes and if found convert it to "!BAR!".
func barEscape(text string) string {
	quote := false
	result := ""

	for _, char := range text {
		if char == '"' {
			quote = !quote
		}

		if char == '|' && quote {
			result += "!BAR!"
		} else {
			result += string(char)
		}
	}

	return result
}

// Search an array of strings and if any contain "!BAR!" convert it back to "|".
func barUnescape(parts []string) []string {
	result := make([]string, len(parts))

	for i, part := range parts {
		result[i] = strings.ReplaceAll(part, "!BAR!", "|")
	}

	return result
}

func makeList(values interface{}, format string) string {
	return strings.Join(makeArray(values, format), ", ")
}

func makeLines(values interface{}, format string) string {
	return strings.Join(makeArray(values, format), "\n")
}

func makeArray(values interface{}, format string) []string {
	var result []string

	switch v := values.(type) {
	case map[string]interface{}:
		format = "%s: " + format
		for key, item := range v {
			result = append(result, fmt.Sprintf(format, key, item))
		}

	case map[string]string:
		format = "%s: " + format
		for key, item := range v {
			result = append(result, fmt.Sprintf(format, key, item))
		}

	case map[interface{}]interface{}:
	case []interface{}:
		for _, item := range v {
			result = append(result, fmt.Sprintf(format, item))
		}

	case []int:
		for _, item := range v {
			result = append(result, fmt.Sprintf(format, item))
		}

	case []int32:
		for _, item := range v {
			result = append(result, fmt.Sprintf(format, item))
		}

	case []int64:
		for _, item := range v {
			result = append(result, fmt.Sprintf(format, item))
		}

	case []float32:
		for _, item := range v {
			result = append(result, fmt.Sprintf(format, item))
		}

	case []float64:
		for _, item := range v {
			result = append(result, fmt.Sprintf(format, item))
		}

	case []string:
		result = v

	default:
		return []string{"!Invalid list type: " + fmt.Sprintf("%T", v) + "!"}
	}

	return result
}

func isZeroValue(value interface{}) bool {
	switch v := value.(type) {
	case []interface{}:
		if len(v) == 0 {
			return true
		}

	case []int:
		if len(v) == 0 {
			return true
		}

	case []string:
		if len(v) == 0 {
			return true
		}

	case map[string]interface{}:
		if len(v) == 0 {
			return true
		}

	case map[string]string:
		if len(v) == 0 {
			return true
		}

	case string:
		if v == "" {
			return true
		}

	case int, int32, int64, float32, float64:
		if v == 0 {
			return true
		}

	case bool:
		if !v {
			return true
		}

	case nil:
		return true
	}

	return false
}

// normalizeNumericValues converts numeric values to be either int or float64 values, based
// on the "wantFloat" flag. This is used to convert JSON-marshalled values (usually float64)
// to expected numeric types for formatting by the substitution processor.
func normalizeNumericValues(value interface{}, wantFloat bool) interface{} {
	switch v := value.(type) {
	case int:
		if wantFloat {
			return float64(v)
		}

		return int(v)

	case int32:
		if wantFloat {
			return float64(v)
		}

		return int(v)

	case int64:
		if wantFloat {
			return float64(v)
		}

		return int(v)

	case float32:
		return normalizeNumericValues(float64(v), wantFloat)

	case float64:
		if wantFloat {
			return float64(v)
		}

		vv := math.Abs(v)
		if vv == math.Floor(vv) {
			return int(v)
		}

		return v

	default:
		return value
	}
}
