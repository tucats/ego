package tasks

import (
	"strings"
	"testing"
)

func TestPatchJSONFieldsNoPatchesReturnsInputUnchanged(t *testing.T) {
	original := []byte(`{"a": 1}`)

	got, err := patchJSONFields(original, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(got) != string(original) {
		t.Errorf("got %q, want unchanged %q", got, original)
	}
}

func TestPatchJSONFieldsReplacesExistingStringField(t *testing.T) {
	original := "{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"active\": \"true\",\n" +
		"\t\"user\": \"admin\"\n" +
		"}\n"

	got, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "active", Value: "false"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := strings.Replace(original, `"active": "true"`, `"active": "false"`, 1)

	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPatchJSONFieldsReplacesExistingIntField(t *testing.T) {
	original := "{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"count\": 4,\n" +
		"\t\"user\": \"admin\"\n" +
		"}\n"

	got, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "count", Value: 12},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := strings.Replace(original, `"count": 4,`, `"count": 12,`, 1)

	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPatchJSONFieldsPreservesUnusualColonSpacing(t *testing.T) {
	// No space after the colon, and extra space before it -- the replaced
	// value must land in exactly the same spot, leaving this idiosyncratic
	// formatting alone.
	original := `{"task":"example","count"  :4}`

	got, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "count", Value: 7},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `{"task":"example","count"  :7}`

	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPatchJSONFieldsInsertsMissingField(t *testing.T) {
	original := "{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"active\": \"true\",\n" +
		"\t\"user\": \"admin\"\n" +
		"}\n"

	got, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "interval", Value: "5m"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"active\": \"true\",\n" +
		"\t\"user\": \"admin\",\n" +
		"\t\"interval\": \"5m\"\n" +
		"}\n"

	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPatchJSONFieldsInsertsMissingIntField(t *testing.T) {
	original := "{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"user\": \"admin\"\n" +
		"}\n"

	got, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "count", Value: 3},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"user\": \"admin\",\n" +
		"\t\"count\": 3\n" +
		"}\n"

	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPatchJSONFieldsInsertMatchesFourSpaceIndent(t *testing.T) {
	original := "{\n" +
		"    \"task\": \"example\",\n" +
		"    \"user\": \"admin\"\n" +
		"}\n"

	got, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "after", Value: "30s"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "{\n" +
		"    \"task\": \"example\",\n" +
		"    \"user\": \"admin\",\n" +
		"    \"after\": \"30s\"\n" +
		"}\n"

	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPatchJSONFieldsMixedReplaceAndInsertAppliedTogether(t *testing.T) {
	original := "{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"active\": \"true\",\n" +
		"\t\"count\": 4,\n" +
		"\t\"user\": \"admin\"\n" +
		"}\n"

	got, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "active", Value: "false"},
		{Key: "count", Value: 9},
		{Key: "interval", Value: "1h"},
		{Key: "after", Value: "10m"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"active\": \"false\",\n" +
		"\t\"count\": 9,\n" +
		"\t\"user\": \"admin\",\n" +
		"\t\"interval\": \"1h\",\n" +
		"\t\"after\": \"10m\"\n" +
		"}\n"

	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPatchJSONFieldsMultipleInsertsPreserveCallerOrder(t *testing.T) {
	// A single-line document has no indented field line to copy, so
	// detectIndent falls back to a single tab -- see the trailing "\t"
	// before each inserted field below.
	original := `{"task": "example", "user": "admin"}`

	got, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "after", Value: "1m"},
		{Key: "interval", Value: "30s"},
		{Key: "count", Value: 4},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "{\"task\": \"example\", \"user\": \"admin\"," +
		"\n\t\"after\": \"1m\"," +
		"\n\t\"interval\": \"30s\"," +
		"\n\t\"count\": 4}"

	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPatchJSONFieldsPreservesHashAndSlashComments(t *testing.T) {
	original := "# a leading comment\n" +
		"// another style of comment\n" +
		"{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"active\": \"true\",\n" +
		"\t\"user\": \"admin\"\n" +
		"}\n"

	got, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "active", Value: "false"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := strings.Replace(original, `"active": "true"`, `"active": "false"`, 1)

	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}

	if !strings.Contains(string(got), "# a leading comment") {
		t.Error("leading '#' comment was not preserved")
	}

	if !strings.Contains(string(got), "// another style of comment") {
		t.Error("leading '//' comment was not preserved")
	}
}

func TestPatchJSONFieldsInsertAfterCommentsUsesCorrectIndent(t *testing.T) {
	original := "# describes this task\n" +
		"{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"user\": \"admin\"\n" +
		"}\n"

	got, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "count", Value: 2},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "# describes this task\n" +
		"{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"user\": \"admin\",\n" +
		"\t\"count\": 2\n" +
		"}\n"

	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestPatchJSONFieldsIgnoresNestedFieldWithSameName is the key correctness
// guarantee of this whole function: a same-named key nested inside another
// top-level field's value (here, "count" inside "body", mimicking a task
// whose outbound request body happens to carry its own "count" field) must
// be left completely alone. A plain string/regex replace would have no way
// to tell this nested "count" apart from the task's own top-level "count"
// field and could corrupt the task's request body.
func TestPatchJSONFieldsIgnoresNestedFieldWithSameName(t *testing.T) {
	original := "{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"body\": {\"count\": 999, \"active\": \"nested-value\"},\n" +
		"\t\"count\": 4\n" +
		"}\n"

	got, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "count", Value: 9},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"body\": {\"count\": 999, \"active\": \"nested-value\"},\n" +
		"\t\"count\": 9\n" +
		"}\n"

	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestPatchJSONFieldsIgnoresNestedFieldWithSameNameArrayValue is the same
// guarantee as above, but for a field nested inside an array of objects
// (mimicking "tests", which is an array), to confirm array nesting is
// tracked correctly too, not just object nesting.
func TestPatchJSONFieldsIgnoresNestedFieldWithSameNameArrayValue(t *testing.T) {
	original := "{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"tests\": [{\"name\": \"x\", \"after\": \"nested\"}],\n" +
		"\t\"after\": \"1m\"\n" +
		"}\n"

	got, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "after", Value: "2m"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"tests\": [{\"name\": \"x\", \"after\": \"nested\"}],\n" +
		"\t\"after\": \"2m\"\n" +
		"}\n"

	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPatchJSONFieldsInsertWhenTargetFieldNameAppearsOnlyNested(t *testing.T) {
	// "count" only appears nested inside "body" here -- from the top-level
	// object's point of view it is still missing, so patching it must
	// INSERT a new top-level field rather than mistakenly "finding" and
	// editing the nested one.
	original := "{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"body\": {\"count\": 1}\n" +
		"}\n"

	got, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "count", Value: 5},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "{\n" +
		"\t\"task\": \"example\",\n" +
		"\t\"body\": {\"count\": 1},\n" +
		"\t\"count\": 5\n" +
		"}\n"

	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}

	if n := strings.Count(string(got), `"count"`); n != 2 {
		t.Errorf("expected exactly two \"count\" occurrences (the untouched nested one, plus the new top-level one), got %d in: %s", n, got)
	}
}

func TestPatchJSONFieldsEscapesValueRequiringJSONEscaping(t *testing.T) {
	original := `{"task": "example", "interval": "1h"}`

	got, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "interval", Value: `contains "quotes" and \backslash`},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `{"task": "example", "interval": "contains \"quotes\" and \\backslash"}`

	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPatchJSONFieldsMalformedJSONReturnsError(t *testing.T) {
	original := `{"task": "example", "count": }`

	if _, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "count", Value: 1},
	}); err == nil {
		t.Error("expected an error for malformed JSON, got nil")
	}
}

func TestPatchJSONFieldsNonObjectRootReturnsError(t *testing.T) {
	original := `[1, 2, 3]`

	if _, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "count", Value: 1},
	}); err == nil {
		t.Error("expected an error for a non-object root, got nil")
	}
}

func TestPatchJSONFieldsRealisticSampleTaskFile(t *testing.T) {
	// Mirrors the shape (and comment style) of the real, checked-in
	// lib/tasks/sample.json, to exercise the patcher against something
	// close to a real-world file rather than only synthetic minimal cases.
	original := "# Sample scheduled task.\n" +
		"#\n" +
		"# It runs four times, thirty seconds apart.\n" +
		"{\n" +
		"\t\"task\": \"sample task: periodic health check\",\n" +
		"\t\"id\": \"0479ae18-4e6d-4c14-bf9b-6d55872ef32f\",\n" +
		"\t\"active\": \"true\",\n" +
		"\t\"user\": \"admin\",\n" +
		"\t\"method\": \"get\",\n" +
		"\t\"endpoint\": \"/services/up\",\n" +
		"\t\"status\": 200,\n" +
		"\t\"tests\": [\n" +
		"\t\t{\n" +
		"\t\t\t\"name\": \"response instance id matches this server\",\n" +
		"\t\t\t\"query\": \"server.id\",\n" +
		"\t\t\t\"value\": \"{{SESSIONID}}\"\n" +
		"\t\t}\n" +
		"\t],\n" +
		"\t\"after\": \"1m\",\n" +
		"\t\"interval\": \"30s\",\n" +
		"\t\"count\": 4\n" +
		"}\n"

	got, err := patchJSONFields([]byte(original), []jsonFieldPatch{
		{Key: "active", Value: "false"},
		{Key: "count", Value: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := strings.NewReplacer(
		`"active": "true"`, `"active": "false"`,
		`"count": 4`, `"count": 10`,
	).Replace(original)

	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
