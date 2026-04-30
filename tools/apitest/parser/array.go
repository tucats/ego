package parser

import "fmt"

func arrayElement(body interface{}, index int, parts []string, item string) ([]string, error) {
	var result []string

	switch actual := body.(type) {
	case []interface{}:
		if index < 0 || index >= len(actual) {
			return result, fmt.Errorf("Index out of range: %d", index)
		}

		return parse(actual[index], parts[1])

	case []string:
		if index < 0 || index >= len(actual) {
			return result, fmt.Errorf("Index out of range: %d", index)
		}

		return []string{actual[index]}, nil

	case []float64:
		if index < 0 || index >= len(actual) {
			return result, fmt.Errorf("Index out of range: %d", index)
		}

		return []string{fmt.Sprintf("%v", actual[index])}, nil

	case []int:
		if index < 0 || index >= len(actual) {
			return result, fmt.Errorf("Index out of range: %d", index)
		}

		return []string{fmt.Sprintf("%v", actual[index])}, nil

	case []bool:
		if index < 0 || index >= len(actual) {
			return result, fmt.Errorf("Index out of range: %d", index)
		}

		return []string{fmt.Sprintf("%v", actual[index])}, nil

	default:
		return result, fmt.Errorf("Item is not an array: %T", item)
	}
}

func anyArrayElement(body interface{}, parts []string, item string) ([]string, error) {
	var result []string

	switch actual := body.(type) {
	case []interface{}:
		for _, element := range actual {
			if text, err := parse(element, parts[1]); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return result, fmt.Errorf("Array element not found: %v", item)

	case []string:
		for _, element := range actual {
			if text, err := parse(element, parts[1]); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return nil, fmt.Errorf("Array element not found: %v", item)

	case []float64:
		for _, element := range actual {
			if text, err := parse(element, parts[1]); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return result, fmt.Errorf("Array element not found: %v", item)

	case []int:
		for _, element := range actual {
			if text, err := parse(element, parts[1]); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return result, fmt.Errorf("Array element not found: %v", item)

	case []bool:
		for _, element := range actual {
			if text, err := parse(element, parts[1]); err == nil {
				result = append(result, text...)
			}
		}

		if len(result) > 0 {
			return result, nil
		}

		return result, fmt.Errorf("Array element not found: %v", item)

	default:
		return result, fmt.Errorf("Item is not an array: %T", item)
	}
}
