package tasks

import (
	"strconv"
	"strings"

	"github.com/tucats/ego/internal/cli/parser"
)

// validCheckOperators is the set of operators a Check.Operator may name.
// The empty string is included: it means "eq", the default.
var validCheckOperators = map[string]bool{
	"":             true,
	"eq":           true,
	"ne":           true,
	"lt":           true,
	"le":           true,
	"gt":           true,
	"ge":           true,
	"contains":     true,
	"not-contains": true,
	"len":          true,
	"exists":       true,
	"not-exists":   true,
}

// runTests evaluates a task's "tests" block against a response body,
// stopping at the first failing check -- patterned after tools/apitest's
// own response validation, which aborts its "tests" block the same way,
// so at most one check is ever implicated in a failure. Returns (true, "")
// if every check passed, including the trivial case of no checks at all;
// otherwise (false, name) naming the first check that failed.
func runTests(task *Task, body []byte) (bool, string) {
	if len(task.Tests) == 0 {
		return true, ""
	}

	text := string(body)

	for _, check := range task.Tests {
		if !evaluateCheck(text, check) {
			return false, check.Name
		}
	}

	return true, ""
}

// evaluateCheck runs one Check against the response body text and reports
// whether it passed.
func evaluateCheck(text string, check Check) bool {
	expect := substitute(check.Value)

	values, err := parser.GetItems(text, check.Query)

	operator := check.Operator
	if operator == "" {
		operator = "eq"
	}

	if err != nil {
		// A query that doesn't resolve is exactly what "not-exists" is
		// checking for, regardless of the specific reason (missing map
		// key, missing array element, malformed body, ...). Every other
		// operator requires a resolved value to compare, so any of them
		// fails outright on an error.
		return operator == "not-exists"
	}

	switch operator {
	case "not-exists":
		// The query resolved fine, so "not exists" is not satisfied.
		return false

	case "exists":
		return true

	case "len":
		length, convErr := strconv.Atoi(expect)

		return convErr == nil && len(values) == length

	case "eq":
		for _, v := range values {
			if v == expect {
				return true
			}
		}

		return false

	case "ne":
		for _, v := range values {
			if v != expect {
				return true
			}
		}

		return false

	case "contains":
		return len(values) > 0 && strings.Contains(values[0], expect)

	case "not-contains":
		return len(values) > 0 && !strings.Contains(values[0], expect)

	case "lt", "le", "gt", "ge":
		if len(values) == 0 {
			return false
		}

		return compareOrdered(values[0], expect, operator)

	default:
		// Unreachable in practice: validateTask rejects any operator not
		// in validCheckOperators before a task is ever registered.
		return false
	}
}

// compareOrdered compares v to expect using operator, trying an integer
// comparison first, then a float comparison, and finally falling back to a
// plain string comparison -- the same three-way fallback tools/apitest's
// own copy uses, so e.g. "9" still compares correctly as less than "10"
// rather than lexically.
func compareOrdered(v, expect, operator string) bool {
	if iv, err := strconv.Atoi(v); err == nil {
		if ie, err := strconv.Atoi(expect); err == nil {
			return compareOperator(iv, ie, operator)
		}
	}

	if fv, err := strconv.ParseFloat(v, 64); err == nil {
		if fe, err := strconv.ParseFloat(expect, 64); err == nil {
			return compareOperator(fv, fe, operator)
		}
	}

	return compareOperator(v, expect, operator)
}

// compareOperator applies one of the four ordering operators to a pair of
// values of any ordered type, letting compareOrdered share one
// implementation across its int/float/string fallback chain instead of
// tools/apitest's three near-identical copies.
func compareOperator[T int | float64 | string](a, b T, operator string) bool {
	switch operator {
	case "lt":
		return a < b
	case "le":
		return a <= b
	case "gt":
		return a > b
	case "ge":
		return a >= b
	default:
		return false
	}
}
