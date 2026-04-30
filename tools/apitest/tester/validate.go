package tester

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/apitest/defs"
	"github.com/tucats/apitest/dictionary"
	"github.com/tucats/apitest/logging"
	"github.com/tucats/apitest/parser"
)

func validateTest(test *defs.Test) error {
	var (
		ok  bool
		err error
	)

	if logging.Verbose {
		fmt.Println("  Validating response tests")
	}
	// For each test case, validate the text.
	for _, t := range test.Tests {
		if logging.Verbose {
			fmt.Printf("    Validating %s\n", t.Name)
		}

		// Apply the dictionary to the value strings
		expect := dictionary.Apply(t.Value)

		value, err := parser.GetItem(test.Response.Body, t.Expression)
		if err != nil {
			// If the error is that the field we expected was not found and the operator is the
			// "not-exists" operator, this is okay and we just continue to the next test.
			if t.Operator == "not-exists" && strings.Contains(err.Error(), "Map element not found") {
				continue
			}

			return err
		}

		switch t.Operator {
		case "len", "length":
			length, err := strconv.Atoi(expect)
			if err != nil {
				return err
			}

			if len(value) != length {
				return fmt.Errorf("%s, %s: expected length to be %d, got %d", test.Description, t.Name, length, len(value))
			}

		case "not-exists":
			// if we got here, then this is an error because the query expression was valid
			// and did exist, so error.
			return fmt.Errorf("%s, %s: expression/field '%s' was present but should not be", test.Description, t.Name, t.Expression)

		case "exists":
			// no action needed for "exists"
			continue

		case "", "eq", ".eq.", "==", "=", "equals", "equal":
			ok = false

			for _, v := range value {
				if v == expect {
					ok = true

					break
				}
			}

			if ok {
				continue
			}

			return fmt.Errorf("%s, %s: expected equal to '%s', got '%v'", test.Description, t.Name, t.Value, value)

		case "!=", "ne", ".ne.", "<>", "not equal":
			ok = false

			for _, v := range value {
				if v != expect {
					ok = true

					break
				}
			}

			if ok {
				continue
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
