package main

import (
	"fmt"
	"strings"
)

func validateText(test *Test) error {
	var err error

	// For each test case, validate the text.
	for _, t := range test.Tests {
		if verbose {
			fmt.Printf("  Validating %s\n", t.Name)
		}

		// Apply the dictionary to the value strings
		expect := ApplyDictionary(t.Value)

		value, err := GetItem(test.Response.Body, t.Expression)
		if err != nil {
			return err
		}

		switch t.Operator {
		case "", "eq", ".eq.", "==", "=", "equals":
			if value != expect {
				return fmt.Errorf("%s, %s: expected '%s', got '%s'", test.Description, t.Name, t.Value, value)
			}

		case "!=", "ne", ".ne.", "<>":
			if value != expect {
				return fmt.Errorf("%s, %s: expected '%s', got '%s'", test.Description, t.Name, t.Value, value)
			}

		case "contains", "has", ".contains,", ".has.", "includes":
			if !strings.Contains(value, expect) {
				return fmt.Errorf("%s, %s: expected '%s' to contain '%s'", test.Description, t.Name, value, t.Value)
			}

		case "not contains", "!contains", ".not contains,":
			if strings.Contains(value, expect) {
				return fmt.Errorf("%s, %s: expected '%s' to contain '%s'", test.Description, t.Name, value, t.Value)
			}
		default:
			return fmt.Errorf("%s, %s: invalid results comparison operator '%s'", test.Description, t.Name, t.Operator)
		}
	}

	return err
}
