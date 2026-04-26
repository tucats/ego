// Package filepath provides Ego bindings for Go's standard path/filepath package.
// This test file verifies that every function in FilepathPackage:
//
//  1. Is actually present in the package map.
//  2. Has the correct Declaration metadata (name, parameter count, return count,
//     variadic flag, etc.).
//  3. Behaves correctly when its underlying Go function is invoked directly.
//
// Because all of these functions are "native" wrappers around the Go standard
// library, the tests extract the function value from the data.Function struct
// and call it with a type assertion, the same way the bytecode engine does
// through reflection (see bytecode/callNative.go).
package filepath

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/ego/data"
)

// ---- helpers ----------------------------------------------------------------

// mustGetFunction retrieves a named entry from FilepathPackage and asserts that
// it is a data.Function.  If either step fails the test is marked fatal, which
// stops the current sub-test immediately so subsequent assertions can assume a
// valid function value.
func mustGetFunction(t *testing.T, name string) data.Function {
	t.Helper()

	raw, ok := FilepathPackage.Get(name)
	if !ok {
		t.Fatalf("FilepathPackage does not contain %q", name)
	}

	fn, ok := raw.(data.Function)
	if !ok {
		t.Fatalf("FilepathPackage[%q] is %T, want data.Function", name, raw)
	}

	return fn
}

// ---- package-level structural tests ----------------------------------------

// TestFilepathPackageContainsExpectedFunctions verifies that FilepathPackage
// registers exactly the six functions that mirror Go's path/filepath package.
// If a function is accidentally removed or misspelled, this test will catch it
// before the runtime panics on a missing symbol lookup.
func TestFilepathPackageContainsExpectedFunctions(t *testing.T) {
	expected := []string{"Abs", "Base", "Clean", "Dir", "Ext", "Join"}

	for _, name := range expected {
		t.Run(name, func(t *testing.T) {
			_, ok := FilepathPackage.Get(name)
			if !ok {
				t.Errorf("FilepathPackage is missing function %q", name)
			}
		})
	}
}

// TestFilepathPackageName confirms that the package's reported name is "filepath".
// Ego uses the package name to resolve import paths, so a wrong name would cause
// "import filepath" to fail silently.
func TestFilepathPackageName(t *testing.T) {
	if got := FilepathPackage.Name; got != "filepath" {
		t.Errorf("package name = %q, want %q", got, "filepath")
	}
}

// ---- declaration metadata tests ---------------------------------------------

// TestFilepathDeclarationAbs verifies the metadata for Abs:
//
//	func Abs(partialPath string) (string, error)
//
// Abs converts a relative path to an absolute one using the current working
// directory.  It returns an error on platforms where the working directory
// cannot be determined.
func TestFilepathDeclarationAbs(t *testing.T) {
	fn := mustGetFunction(t, "Abs")
	d := fn.Declaration

	if d.Name != "Abs" {
		t.Errorf("Declaration.Name = %q, want %q", d.Name, "Abs")
	}

	if !fn.IsNative {
		t.Error("Abs should be marked IsNative = true")
	}

	if len(d.Parameters) != 1 {
		t.Fatalf("Abs: want 1 parameter, got %d", len(d.Parameters))
	}

	if d.Parameters[0].Name != "partialPath" {
		t.Errorf("Abs parameter[0] name = %q, want %q", d.Parameters[0].Name, "partialPath")
	}

	// Abs returns (string, error).
	if len(d.Returns) != 2 {
		t.Fatalf("Abs: want 2 return values, got %d", len(d.Returns))
	}
}

// TestFilepathDeclarationBase verifies the metadata for Base:
//
//	func Base(path string) string
//
// Base returns the last element of a path, stripping any trailing slashes.
// It behaves like the Unix basename command.
func TestFilepathDeclarationBase(t *testing.T) {
	fn := mustGetFunction(t, "Base")
	d := fn.Declaration

	if d.Name != "Base" {
		t.Errorf("Declaration.Name = %q, want %q", d.Name, "Base")
	}

	if !fn.IsNative {
		t.Error("Base should be marked IsNative = true")
	}

	if len(d.Parameters) != 1 {
		t.Fatalf("Base: want 1 parameter, got %d", len(d.Parameters))
	}

	// Base returns a single string.
	if len(d.Returns) != 1 {
		t.Fatalf("Base: want 1 return value, got %d", len(d.Returns))
	}
}

// TestFilepathDeclarationClean verifies the metadata for Clean:
//
//	func Clean(path string) string
//
// Clean returns the shortest equivalent path by applying lexical rules such as
// eliminating double slashes and resolving "." and ".." elements.
func TestFilepathDeclarationClean(t *testing.T) {
	fn := mustGetFunction(t, "Clean")
	d := fn.Declaration

	if d.Name != "Clean" {
		t.Errorf("Declaration.Name = %q, want %q", d.Name, "Clean")
	}

	if !fn.IsNative {
		t.Error("Clean should be marked IsNative = true")
	}

	if len(d.Parameters) != 1 {
		t.Fatalf("Clean: want 1 parameter, got %d", len(d.Parameters))
	}

	if len(d.Returns) != 1 {
		t.Fatalf("Clean: want 1 return value, got %d", len(d.Returns))
	}
}

// TestFilepathDeclarationDir verifies the metadata for Dir:
//
//	func Dir(path string) string
//
// Dir returns everything except the last element of a path, behaving like
// the Unix dirname command.
func TestFilepathDeclarationDir(t *testing.T) {
	fn := mustGetFunction(t, "Dir")
	d := fn.Declaration

	if d.Name != "Dir" {
		t.Errorf("Declaration.Name = %q, want %q", d.Name, "Dir")
	}

	if !fn.IsNative {
		t.Error("Dir should be marked IsNative = true")
	}

	if len(d.Parameters) != 1 {
		t.Fatalf("Dir: want 1 parameter, got %d", len(d.Parameters))
	}

	if len(d.Returns) != 1 {
		t.Fatalf("Dir: want 1 return value, got %d", len(d.Returns))
	}
}

// TestFilepathDeclarationExt verifies the metadata for Ext:
//
//	func Ext(path string) string
//
// Ext returns the file name extension: the suffix from the last "." in the
// last element of path.  An empty string is returned when there is no
// extension.
func TestFilepathDeclarationExt(t *testing.T) {
	fn := mustGetFunction(t, "Ext")
	d := fn.Declaration

	if d.Name != "Ext" {
		t.Errorf("Declaration.Name = %q, want %q", d.Name, "Ext")
	}

	if !fn.IsNative {
		t.Error("Ext should be marked IsNative = true")
	}

	if len(d.Parameters) != 1 {
		t.Fatalf("Ext: want 1 parameter, got %d", len(d.Parameters))
	}

	if len(d.Returns) != 1 {
		t.Fatalf("Ext: want 1 return value, got %d", len(d.Returns))
	}
}

// TestFilepathDeclarationJoin verifies the metadata for Join:
//
//	func Join(elements ...string) string
//
// Join concatenates path elements with the OS-specific separator.  It also
// calls Clean on the result, so redundant separators and ".." references are
// eliminated.  The Variadic flag must be true so the bytecode engine passes
// all arguments as a slice.
func TestFilepathDeclarationJoin(t *testing.T) {
	fn := mustGetFunction(t, "Join")
	d := fn.Declaration

	if d.Name != "Join" {
		t.Errorf("Declaration.Name = %q, want %q", d.Name, "Join")
	}

	if !fn.IsNative {
		t.Error("Join should be marked IsNative = true")
	}

	if !d.Variadic {
		t.Error("Join Declaration.Variadic should be true")
	}

	if len(d.Parameters) != 1 {
		t.Fatalf("Join: want 1 variadic parameter, got %d", len(d.Parameters))
	}

	// Join returns a single string (the joined path).
	if len(d.Returns) != 1 {
		t.Fatalf("Join: want 1 return value, got %d", len(d.Returns))
	}
}

// ---- function-value behavior tests ----------------------------------------
//
// Each test below extracts the underlying Go function from data.Function.Value
// and calls it directly.  This verifies that the Value field actually holds the
// expected standard library function and that the function produces correct results.
//
// The type assertions (e.g.  fn.Value.(func(string) string)) mirror the type
// the bytecode reflection engine expects when it calls these via reflect.Value.
// If a wrong function were accidentally wired up, the assertion would panic and
// the test would fail with a clear message.

// TestFilepathBaseValues exercises Base() with a variety of inputs and checks
// that the result matches Go's standard library directly.
func TestFilepathBaseValues(t *testing.T) {
	fn := mustGetFunction(t, "Base")
	base, ok := fn.Value.(func(string) string)

	if !ok {
		t.Fatalf("Base value is %T, want func(string) string", fn.Value)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Typical Unix path: last element only.
		{name: "unix path", input: "/usr/local/bin/ego", want: "ego"},
		// Trailing slash: slash is stripped before taking the base.
		{name: "trailing slash", input: "/tmp/", want: "tmp"},
		// Single file name with no directory component.
		{name: "filename only", input: "readme.md", want: "readme.md"},
		// Dot is a valid path meaning "current directory".
		{name: "dot", input: ".", want: "."},
		// An empty path is defined to return ".".
		{name: "empty string", input: "", want: "."},
		// Path with extension: extension is part of the base name.
		{name: "file with extension", input: "/home/user/notes.txt", want: "notes.txt"},
		// Multiple consecutive slashes are treated as one separator.
		{name: "double slash", input: "//a//b", want: "b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := base(tt.input)
			if got != tt.want {
				t.Errorf("Base(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestFilepathDirValues exercises Dir() with typical Unix paths.
func TestFilepathDirValues(t *testing.T) {
	fn := mustGetFunction(t, "Dir")
	dir, ok := fn.Value.(func(string) string)

	if !ok {
		t.Fatalf("Dir value is %T, want func(string) string", fn.Value)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Normal path: everything except the last element.
		{name: "unix path", input: "/usr/local/bin/ego", want: "/usr/local/bin"},
		// Single level: the directory is the root.
		{name: "root file", input: "/ego", want: "/"},
		// No directory: just a file name, so the "directory" is ".".
		{name: "filename only", input: "readme.md", want: "."},
		// Dot input: Dir(".") = ".".
		{name: "dot", input: ".", want: "."},
		// Empty string: Dir("") = ".".
		{name: "empty string", input: "", want: "."},
		// Go's Dir finds the last path separator character, so for "/tmp/" the
		// last separator is the trailing slash; the directory portion is "/tmp".
		// This differs from what you might expect after a Clean pass.
		{name: "trailing slash", input: "/tmp/", want: "/tmp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dir(tt.input)
			if got != tt.want {
				t.Errorf("Dir(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestFilepathExtValues exercises Ext() and verifies extension extraction.
func TestFilepathExtValues(t *testing.T) {
	fn := mustGetFunction(t, "Ext")
	ext, ok := fn.Value.(func(string) string)

	if !ok {
		t.Fatalf("Ext value is %T, want func(string) string", fn.Value)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Standard file extension.
		{name: "go file", input: "main.go", want: ".go"},
		// The dot is included in the returned extension.
		{name: "text file", input: "readme.txt", want: ".txt"},
		// No extension: returns empty string.
		{name: "no extension", input: "Makefile", want: ""},
		// The extension is taken from the last dot in the base name only.
		{name: "multiple dots", input: "archive.tar.gz", want: ".gz"},
		// Go's Ext scans backwards for any dot; the leading dot in a dotfile IS
		// treated as an extension separator, so ".bashrc" returns ".bashrc".
		{name: "dotfile", input: ".bashrc", want: ".bashrc"},
		// Empty path: no extension.
		{name: "empty string", input: "", want: ""},
		// Full path: extension is from the base name.
		{name: "full path", input: "/home/user/doc.pdf", want: ".pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ext(tt.input)
			if got != tt.want {
				t.Errorf("Ext(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestFilepathJoinValues exercises Join() with varying numbers of arguments.
// Join calls Clean on its result, so redundant separators are eliminated.
func TestFilepathJoinValues(t *testing.T) {
	fn := mustGetFunction(t, "Join")
	join, ok := fn.Value.(func(...string) string)

	if !ok {
		t.Fatalf("Join value is %T, want func(...string) string", fn.Value)
	}

	tests := []struct {
		name  string
		input []string
		want  string
	}{
		// Two simple segments.
		{name: "two segments", input: []string{"/usr", "local"}, want: "/usr/local"},
		// Three segments.
		{name: "three segments", input: []string{"/a", "b", "c"}, want: "/a/b/c"},
		// An empty segment in the middle is ignored by Clean.
		{name: "empty middle segment", input: []string{"/a", "", "b"}, want: "/a/b"},
		// All-empty input collapses to an empty string.
		{name: "all empty", input: []string{"", ""}, want: ""},
		// Single segment: nothing to join.
		{name: "single segment", input: []string{"/etc"}, want: "/etc"},
		// ".." is resolved by the implicit Clean.
		{name: "dot-dot resolved", input: []string{"/a/b", "..", "c"}, want: "/a/c"},
		// No arguments: returns empty string.
		{name: "no arguments", input: []string{}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := join(tt.input...)
			if got != tt.want {
				t.Errorf("Join(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestFilepathCleanValues exercises Clean() path normalization rules.
// The rules are applied lexically without touching the file system:
// double slashes are removed, "." elements are removed, ".." elements
// collapse their parent, and a trailing slash is stripped.
func TestFilepathCleanValues(t *testing.T) {
	fn := mustGetFunction(t, "Clean")
	clean, ok := fn.Value.(func(string) string)

	if !ok {
		t.Fatalf("Clean value is %T, want func(string) string", fn.Value)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		// Already clean: no change.
		{name: "already clean", input: "/usr/local/bin", want: "/usr/local/bin"},
		// Double slash collapsed.
		{name: "double slash", input: "//usr//local", want: "/usr/local"},
		// Trailing slash removed.
		{name: "trailing slash", input: "/tmp/", want: "/tmp"},
		// Dot in the middle removed.
		{name: "dot in middle", input: "/a/./b", want: "/a/b"},
		// Double-dot collapses parent.
		{name: "dot-dot", input: "/a/b/../c", want: "/a/c"},
		// Empty string becomes ".".
		{name: "empty string", input: "", want: "."},
		// Purely relative double-dot stays as "..".
		{name: "relative dot-dot", input: "..", want: ".."},
		// Multiple redundant separators and dots together.
		{name: "complex", input: "/a//b/./c/../d", want: "/a/b/d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clean(tt.input)
			if got != tt.want {
				t.Errorf("Clean(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestFilepathAbsValues exercises Abs() for a handful of typical inputs.
// Because Abs depends on the process working directory, absolute paths are
// verified to be unchanged, and relative paths are verified against an
// expected suffix derived from filepath.Join(cwd, rel).
func TestFilepathAbsValues(t *testing.T) {
	fn := mustGetFunction(t, "Abs")
	abs, ok := fn.Value.(func(string) (string, error))

	if !ok {
		t.Fatalf("Abs value is %T, want func(string) (string, error)", fn.Value)
	}

	// Get the current working directory so we can construct expected results
	// for relative paths without hard-coding a machine-specific path.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() failed: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		want    string // empty means "derive from cwd"
		wantErr bool
	}{
		// An already-absolute path must be returned unchanged (after Clean).
		{name: "already absolute", input: "/usr/local/bin", want: "/usr/local/bin"},
		// A dot resolves to the current working directory.
		{name: "dot", input: ".", want: cwd},
		// A relative single name resolves inside the working directory.
		{name: "relative name", input: "ego", want: filepath.Join(cwd, "ego")},
		// ".." goes up one level from the working directory.
		{name: "dot-dot", input: "..", want: filepath.Dir(cwd)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := abs(tt.input)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Abs(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}

			if !tt.wantErr && got != tt.want {
				t.Errorf("Abs(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestFilepathAbsReturnsAbsolutePath is a property-based sanity check: whatever
// Abs returns for any input must itself be an absolute path (i.e. it starts with
// the OS path separator).  This holds regardless of the current directory.
func TestFilepathAbsReturnsAbsolutePath(t *testing.T) {
	fn := mustGetFunction(t, "Abs")
	abs := fn.Value.(func(string) (string, error))

	inputs := []string{".", "..", "ego", "a/b/c", "/already/absolute"}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			got, err := abs(input)
			if err != nil {
				t.Fatalf("Abs(%q) unexpected error: %v", input, err)
			}

			if !filepath.IsAbs(got) {
				t.Errorf("Abs(%q) = %q is not an absolute path", input, got)
			}
		})
	}
}
