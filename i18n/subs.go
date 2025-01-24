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

	value = normalizeNumericValues(value)

	// Check for special cases in the format string
	formatParts := strings.Split(format, "|")
	label := ""
	format = "%v"

	for _, part := range formatParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		switch {
		case strings.HasPrefix(part, "%"):
			format = part

		case strings.HasPrefix(part, "label"):
			if !isZeroValue(value) {
				label = strings.TrimSpace(part[len("label"):])

				if unquoted, err := strconv.Unquote(label); err == nil {
					label = unquoted
				}
			} else {
				label = ""
				format = "%s"
				value = ""
			}

		case strings.HasPrefix(part, "format"):
			format = strings.TrimSpace(part[len("format"):])

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
			value = makeList(value)
			format = "%s"

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

	result := fmt.Sprintf("%s"+format, label, value)

	return result
}

func makeList(values interface{}) string {
	var result []string

	switch v := values.(type) {
	case map[string]interface{}:
		for key, item := range v {
			result = append(result, fmt.Sprintf("%s: %v", key, item))
		}

	case map[string]string:
		for key, item := range v {
			result = append(result, fmt.Sprintf("%s: %s", key, item))
		}

	case map[interface{}]interface{}:
	case []interface{}:
		for _, item := range v {
			result = append(result, fmt.Sprintf("%v", item))
		}

	case []int:
		for _, item := range v {
			result = append(result, strconv.Itoa(item))
		}

	case []int32:
		for _, item := range v {
			result = append(result, strconv.Itoa(int(item)))
		}

	case []int64:
		for _, item := range v {
			result = append(result, strconv.FormatInt(item, 10))
		}

	case []float32:
		for _, item := range v {
			result = append(result, strconv.FormatFloat(float64(item), 'g', 8, 32))
		}

	case []float64:
		for _, item := range v {
			result = append(result, strconv.FormatFloat(item, 'g', 10, 64))
		}

	case []string:
		result = v

	default:
		return "!Invalid list type: " + fmt.Sprintf("%T", v) + "!"
	}

	return strings.Join(result, ", ")
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

func normalizeNumericValues(value interface{}) interface{} {
	switch v := value.(type) {
	case int:
		return int(v)

	case int32:
		return int(v)

	case int64:
		return int(v)

	case float32:
		return normalizeNumericValues(float64(v))

	case float64:
		vv := math.Abs(v)
		if vv == math.Floor(vv) {
			return int(v)
		}

		return v

	default:
		return value
	}
}
