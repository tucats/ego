package tasks

import "testing"

const sampleBody = `{
	"server": {"id": "abc-123", "session": 4},
	"count": 3,
	"items": [{"name": "a"}, {"name": "b"}, {"name": "c"}],
	"message": "hello world"
}`

func TestRunTestsNoChecksAlwaysPasses(t *testing.T) {
	task := &Task{}

	ok, name := runTests(task, []byte(sampleBody))
	if !ok || name != "" {
		t.Errorf("runTests() = (%v, %q), want (true, \"\")", ok, name)
	}
}

func TestRunTestsAllPass(t *testing.T) {
	task := &Task{Tests: []Check{
		{Name: "server id", Query: "server.id", Value: "abc-123", Operator: "eq"},
		{Name: "count", Query: "count", Value: "3"},
		{Name: "message contains", Query: "message", Value: "world", Operator: "contains"},
	}}

	ok, name := runTests(task, []byte(sampleBody))
	if !ok || name != "" {
		t.Errorf("runTests() = (%v, %q), want (true, \"\")", ok, name)
	}
}

func TestRunTestsStopsAtFirstFailure(t *testing.T) {
	task := &Task{Tests: []Check{
		{Name: "first ok", Query: "count", Value: "3"},
		{Name: "second fails", Query: "count", Value: "999"},
		{Name: "third never evaluated", Query: "message", Value: "does-not-matter"},
	}}

	ok, name := runTests(task, []byte(sampleBody))
	if ok || name != "second fails" {
		t.Errorf("runTests() = (%v, %q), want (false, %q)", ok, name, "second fails")
	}
}

func TestRunTestsValueIsSubstituted(t *testing.T) {
	resetSaved(t)
	setSaved("EXPECTED_ID", "abc-123")

	task := &Task{Tests: []Check{
		{Name: "substituted value", Query: "server.id", Value: "{{EXPECTED_ID}}"},
	}}

	ok, _ := runTests(task, []byte(sampleBody))
	if !ok {
		t.Error("expected the check to pass once {{EXPECTED_ID}} substitutes to the matching value")
	}
}

func TestEvaluateCheckOperators(t *testing.T) {
	tests := []struct {
		name  string
		check Check
		want  bool
	}{
		{"eq default operator match", Check{Query: "count", Value: "3"}, true},
		{"eq default operator mismatch", Check{Query: "count", Value: "4"}, false},
		{"eq explicit", Check{Query: "server.id", Value: "abc-123", Operator: "eq"}, true},
		{"ne match", Check{Query: "server.id", Value: "different", Operator: "ne"}, true},
		{"ne mismatch", Check{Query: "server.id", Value: "abc-123", Operator: "ne"}, false},
		{"lt true (numeric)", Check{Query: "count", Value: "10", Operator: "lt"}, true},
		{"lt false (numeric)", Check{Query: "count", Value: "1", Operator: "lt"}, false},
		{"le equal", Check{Query: "count", Value: "3", Operator: "le"}, true},
		{"gt true", Check{Query: "count", Value: "1", Operator: "gt"}, true},
		{"gt false", Check{Query: "count", Value: "3", Operator: "gt"}, false},
		{"ge equal", Check{Query: "count", Value: "3", Operator: "ge"}, true},
		{"contains true", Check{Query: "message", Value: "hello", Operator: "contains"}, true},
		{"contains false", Check{Query: "message", Value: "goodbye", Operator: "contains"}, false},
		{"not-contains true", Check{Query: "message", Value: "goodbye", Operator: "not-contains"}, true},
		{"not-contains false", Check{Query: "message", Value: "hello", Operator: "not-contains"}, false},
		{"exists true", Check{Query: "server.id", Operator: "exists"}, true},
		{"exists false", Check{Query: "no.such.field", Operator: "exists"}, false},
		{"not-exists true", Check{Query: "no.such.field", Operator: "not-exists"}, true},
		{"not-exists false", Check{Query: "server.id", Operator: "not-exists"}, false},
		{"len bare array", Check{Query: "items", Value: "3", Operator: "len"}, true},
		{"len bare array wrong count", Check{Query: "items", Value: "2", Operator: "len"}, false},
		{"len wildcard array", Check{Query: "items.*.name", Value: "3", Operator: "len"}, true},
		{"len non-numeric value is a failure, not a panic", Check{Query: "items", Value: "not-a-number", Operator: "len"}, false},
		{"unresolvable query with eq fails, not panics", Check{Query: "no.such.field", Value: "x", Operator: "eq"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := evaluateCheck(sampleBody, tc.check); got != tc.want {
				t.Errorf("evaluateCheck(%+v) = %v, want %v", tc.check, got, tc.want)
			}
		})
	}
}

func TestEvaluateCheckNumericFallbackToFloatThenString(t *testing.T) {
	body := `{"pi": "3.14", "word": "banana"}`

	if !evaluateCheck(body, Check{Query: "pi", Value: "3.0", Operator: "gt"}) {
		t.Error("expected float fallback comparison 3.14 > 3.0 to pass")
	}

	if !evaluateCheck(body, Check{Query: "word", Value: "apple", Operator: "gt"}) {
		t.Error("expected string fallback comparison \"banana\" > \"apple\" to pass")
	}
}

func TestCompareOperator(t *testing.T) {
	if !compareOperator(1, 2, "lt") {
		t.Error("1 lt 2 should be true")
	}

	if compareOperator(2, 2, "lt") {
		t.Error("2 lt 2 should be false")
	}

	if !compareOperator(2.0, 2.0, "le") {
		t.Error("2.0 le 2.0 should be true")
	}

	if !compareOperator("b", "a", "gt") {
		t.Error("\"b\" gt \"a\" should be true")
	}

	if compareOperator(1, 1, "unknown-operator") {
		t.Error("an unrecognized operator should default to false")
	}
}
