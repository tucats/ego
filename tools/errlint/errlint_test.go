package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()

	path := filepath.Join(dir, name)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	return path
}

func TestExtractErrorDefsIndividual(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "messages.go", `package errors

var ErrFoo = Message("foo.bar")
var ErrBaz = Message("baz.qux")

// not an error definition
var notAnError = 42
`)

	defs, err := extractErrorDefs(path)
	if err != nil {
		t.Fatalf("extractErrorDefs: %v", err)
	}

	want := map[string]string{"ErrFoo": "foo.bar", "ErrBaz": "baz.qux"}

	if len(defs) != len(want) {
		t.Fatalf("got %d defs, want %d: %+v", len(defs), len(want), defs)
	}

	for _, d := range defs {
		if want[d.Symbol] != d.Key {
			t.Errorf("symbol %s: got key %q, want %q", d.Symbol, d.Key, want[d.Symbol])
		}
	}
}

func TestExtractErrorDefsGroupedAndQualified(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "messages.go", `package other

import "github.com/tucats/ego/internal/errors"

var (
	ErrOne = errors.Message("one")
	ErrTwo = errors.Message("two")
)
`)

	defs, err := extractErrorDefs(path)
	if err != nil {
		t.Fatalf("extractErrorDefs: %v", err)
	}

	want := map[string]string{"ErrOne": "one", "ErrTwo": "two"}

	if len(defs) != len(want) {
		t.Fatalf("got %d defs, want %d: %+v", len(defs), len(want), defs)
	}

	for _, d := range defs {
		if want[d.Symbol] != d.Key {
			t.Errorf("symbol %s: got key %q, want %q", d.Symbol, d.Key, want[d.Symbol])
		}
	}
}

func TestLoadCatalogKeys(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "messages_en.txt", `# a comment
bare.key=no section

[error]
foo.bar=Foo happened: {{value}}
baz.qux=Baz went wrong

[log]
foo.bar=unrelated log message with the same leaf key
`)

	keys, err := loadCatalogKeys(path)
	if err != nil {
		t.Fatalf("loadCatalogKeys: %v", err)
	}

	for _, want := range []string{"bare.key", "error.foo.bar", "error.baz.qux", "log.foo.bar"} {
		if !keys[want] {
			t.Errorf("missing expected key %q; got %v", want, keys)
		}
	}

	if len(keys) != 4 {
		t.Errorf("got %d keys, want 4: %v", len(keys), keys)
	}
}

func TestFindUsedSymbols(t *testing.T) {
	dir := t.TempDir()

	defsPath := writeTestFile(t, dir, "errors/messages.go", `package errors

var ErrFoo = Message("foo")
var ErrUnused = Message("unused")
`)

	writeTestFile(t, dir, "consumer/consumer.go", `package consumer

import "github.com/tucats/ego/internal/errors"

func doSomething() error {
	return errors.ErrFoo
}
`)

	abs, err := filepath.Abs(defsPath)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}

	used, err := findUsedSymbols(dir, map[string]bool{abs: true})
	if err != nil {
		t.Fatalf("findUsedSymbols: %v", err)
	}

	if !used["ErrFoo"] {
		t.Errorf("expected ErrFoo to be reported as used")
	}

	if used["ErrUnused"] {
		t.Errorf("did not expect ErrUnused to be reported as used")
	}
}

func TestFindUsedSymbolsSkipsDefinitionFile(t *testing.T) {
	dir := t.TempDir()

	defsPath := writeTestFile(t, dir, "errors/messages.go", `package errors

var ErrFoo = Message("foo")
`)

	abs, err := filepath.Abs(defsPath)
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}

	used, err := findUsedSymbols(dir, map[string]bool{abs: true})
	if err != nil {
		t.Fatalf("findUsedSymbols: %v", err)
	}

	if used["ErrFoo"] {
		t.Errorf("did not expect ErrFoo to be reported as used, since its only appearance is in the skipped definition file")
	}
}

func TestBuildReportUnusedAndMissing(t *testing.T) {
	defs := []errorDef{
		{Symbol: "ErrUsedAndLocalized", Key: "used.ok", File: "messages.go", Line: 1},
		{Symbol: "ErrUnused", Key: "unused.ok", File: "messages.go", Line: 2},
		{Symbol: "ErrNotLocalized", Key: "not.localized", File: "messages.go", Line: 3},
		{Symbol: "ErrSentinel", Key: "_sentinel", File: "messages.go", Line: 4},
	}

	catalog := map[string]bool{
		"error.used.ok":   true,
		"error.unused.ok": true,
	}

	used := map[string]bool{
		"ErrUsedAndLocalized": true,
		"ErrNotLocalized":     true,
		"ErrSentinel":         true,
	}

	rep := buildReport(defs, catalog, used)

	if len(rep.unused) != 1 || rep.unused[0].Symbol != "ErrUnused" {
		t.Errorf("unexpected unused list: %+v", rep.unused)
	}

	if len(rep.missing) != 1 || rep.missing[0].Symbol != "ErrNotLocalized" {
		t.Errorf("unexpected missing list: %+v", rep.missing)
	}

	if len(rep.duplicates) != 0 {
		t.Errorf("unexpected duplicates: %v", rep.duplicates)
	}
}

func TestBuildReportSentinelExemptFromLocalizationOnly(t *testing.T) {
	defs := []errorDef{
		{Symbol: "ErrContinue", Key: "_continue", File: "messages.go", Line: 1},
	}

	rep := buildReport(defs, map[string]bool{}, map[string]bool{})

	if len(rep.missing) != 0 {
		t.Errorf("sentinel key should be exempt from localization check, got missing: %+v", rep.missing)
	}

	if len(rep.unused) != 1 {
		t.Errorf("sentinel key should still be checked for usage, got unused: %+v", rep.unused)
	}
}

func TestBuildReportDuplicateSymbol(t *testing.T) {
	defs := []errorDef{
		{Symbol: "ErrDup", Key: "dup.one", File: "a.go", Line: 1},
		{Symbol: "ErrDup", Key: "dup.two", File: "b.go", Line: 2},
	}

	rep := buildReport(defs, map[string]bool{"error.dup.one": true}, map[string]bool{"ErrDup": true})

	if len(rep.duplicates) != 1 {
		t.Fatalf("expected 1 duplicate, got %d: %v", len(rep.duplicates), rep.duplicates)
	}

	if len(rep.unused) != 0 || len(rep.missing) != 0 {
		t.Errorf("duplicate symbol should not also be reported unused/missing: unused=%+v missing=%+v", rep.unused, rep.missing)
	}
}

func TestParseArgumentsRequiresAllOptions(t *testing.T) {
	if _, err := parseArguments([]string{}); err == nil {
		t.Errorf("expected error for missing options")
	}

	opts, err := parseArguments([]string{
		"--errors", "a.go",
		"--errors", "b.go",
		"--strings", "messages_en.txt",
		"--source", ".",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(opts.errorFiles) != 2 || len(opts.stringFiles) != 1 || len(opts.sourcePaths) != 1 {
		t.Errorf("unexpected parsed options: %+v", opts)
	}
}
