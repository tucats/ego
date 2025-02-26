package main

import (
	"fmt"
	"strconv"
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
		case "", "eq", ".eq.", "==", "=", "equals", "equal":
			for _, v := range value {
				if v == expect {
					return nil
				}
			}

			return fmt.Errorf("%s, %s: expected '%s', got '%s'", test.Description, t.Name, t.Value, value)

		case "!=", "ne", ".ne.", "<>", "not equal":
			for _, v := range value {
				if v != expect {
					return nil
				}
			}

			return fmt.Errorf("%s, %s: expected not finding'%s', got '%s'", test.Description, t.Name, t.Value, value)

		case "<", "lt", ".lt.", "less than":
			// See if this can be done as an integer comparison.
			if len(value) == 0 {
				return fmt.Errorf("%s, %s: expected a value, found none", test.Description, t.Name)
			}

			v := value[0]

			if iValue, err := strconv.Atoi(v); err == nil {
				if iExpect, err := strconv.Atoi(expect); err == nil {
					if iValue >= iExpect {
						return fmt.Errorf("%s, %s: expected '%s' to be less than '%s'", test.Description, t.Name, value, expect)
					} else {
						continue
					}
				}
			}

			// See if this can be done as an float comparison.
			if fValue, err := strconv.ParseFloat(v, 64); err == nil {
				if fExpect, err := strconv.ParseFloat(expect, 64); err == nil {
					if fValue >= fExpect {
						return fmt.Errorf("%s, %s: expected '%s' to be less than '%s'", test.Description, t.Name, value, expect)
					} else {
						continue
					}
				}
			}

			// If not, just do string comparison.
			if v >= expect {
				return fmt.Errorf("%s, %s: expected '%s' to be less than '%s'", test.Description, t.Name, value, expect)
			}

		case "<=", "le", ".le.", "less than or equal":
			if len(value) == 0 {
				return fmt.Errorf("%s, %s: expected a value, found none", test.Description, t.Name)
			}

			v := value[0]

			// See if this can be done as an integer comparison.
			if iValue, err := strconv.Atoi(v); err == nil {
				if iExpect, err := strconv.Atoi(expect); err == nil {
					if iValue > iExpect {
						return fmt.Errorf("%s, %s: expected '%s' to be less than or equal to '%s'", test.Description, t.Name, value, expect)
					} else {
						continue
					}
				}
			}

			// See if this can be done as an float comparison.
			if fValue, err := strconv.ParseFloat(v, 64); err == nil {
				if fExpect, err := strconv.ParseFloat(expect, 64); err == nil {
					if fValue > fExpect {
						return fmt.Errorf("%s, %s: expected '%s' to be less than or equal to '%s'", test.Description, t.Name, value, expect)
					} else {
						continue
					}
				}
			}

			// If not, just do string comparison.
			if v < expect {
				return fmt.Errorf("%s, %s: expected '%s' to be less than or equal to '%s'", test.Description, t.Name, value, expect)
			}

		case ">=", "ge", ".ge.", "greater than or equal":
			if len(value) == 0 {
				return fmt.Errorf("%s, %s: expected a value, found none", test.Description, t.Name)
			}

			v := value[0]

			// See if this can be done as an integer comparison.
			if iValue, err := strconv.Atoi(v); err == nil {
				if iExpect, err := strconv.Atoi(expect); err == nil {
					if iValue < iExpect {
						return fmt.Errorf("%s, %s: expected '%s' to be greater than or equal to '%s'", test.Description, t.Name, value, expect)
					} else {
						continue
					}
				}
			}

			// See if this can be done as an float comparison.
			if fValue, err := strconv.ParseFloat(v, 64); err == nil {
				if fExpect, err := strconv.ParseFloat(expect, 64); err == nil {
					if fValue < fExpect {
						return fmt.Errorf("%s, %s: expected '%s' to be greater than or equal to '%s'", test.Description, t.Name, value, expect)
					} else {
						continue
					}
				}
			}

			// If not, just do string comparison.
			if v < expect {
				return fmt.Errorf("%s, %s: expected '%s' to be greater than or equal to '%s'", test.Description, t.Name, value, expect)
			}

		case ">", "gt", ".gt.", "greater than":
			if len(value) == 0 {
				return fmt.Errorf("%s, %s: expected a value, found none", test.Description, t.Name)
			}

			v := value[0]

			// See if this can be done as an integer comparison.
			if iValue, err := strconv.Atoi(v); err == nil {
				if iExpect, err := strconv.Atoi(expect); err == nil {
					if iValue <= iExpect {
						return fmt.Errorf("%s, %s: expected '%s' to be greater than '%s'", test.Description, t.Name, value, expect)
					} else {
						continue
					}
				}
			}

			// See if this can be done as an float comparison.
			if fValue, err := strconv.ParseFloat(v, 64); err == nil {
				if fExpect, err := strconv.ParseFloat(expect, 64); err == nil {
					if fValue <= fExpect {
						return fmt.Errorf("%s, %s: expected '%s' to be greater than '%s'", test.Description, t.Name, value, expect)
					} else {
						continue
					}
				}
			}

			// If not, just do string comparison.
			if v <= expect {
				return fmt.Errorf("%s, %s: expected '%s' to be greater than '%s'", test.Description, t.Name, value, expect)
			}

		case "contains", "has", ".contains,", ".has.", "includes":
			if len(value) == 0 {
				return fmt.Errorf("%s, %s: expected a value, found none", test.Description, t.Name)
			}

			v := value[0]

			if !strings.Contains(v, expect) {
				return fmt.Errorf("%s, %s: expected '%s' to contain '%s'", test.Description, t.Name, value, t.Value)
			}

		case "not contains", "!contains", ".not contains,":
			if len(value) == 0 {
				return fmt.Errorf("%s, %s: expected a value, found none", test.Description, t.Name)
			}

			v := value[0]

			if strings.Contains(v, expect) {
				return fmt.Errorf("%s, %s: expected '%s' to contain '%s'", test.Description, t.Name, value, t.Value)
			}
		default:
			return fmt.Errorf("%s, %s: invalid results comparison operator '%s'", test.Description, t.Name, t.Operator)
		}
	}

	return err
}
