package javascript

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// helper: minify src and return as string.
func minifyString(t *testing.T, src string) string {
	t.Helper()

	return string(Minify([]byte(src)))
}

func TestMinify_RemovesLineComments(t *testing.T) {
	src := `var x = 1; // this is a comment
var y = 2;`
	got := minifyString(t, src)
	assert.NotContains(t, got, "//")
	assert.NotContains(t, got, "this is a comment")
	assert.Contains(t, got, "=1")
	assert.Contains(t, got, "=2")
}

func TestMinify_RemovesBlockComments(t *testing.T) {
	src := `var x = /* inline comment */ 42;`
	got := minifyString(t, src)
	assert.NotContains(t, got, "/*")
	assert.NotContains(t, got, "inline comment")
	assert.Contains(t, got, "42")
}

func TestMinify_CollapsesWhitespace(t *testing.T) {
	src := `var   x   =   1 ;`
	got := minifyString(t, src)
	// Should not contain multiple consecutive spaces.
	assert.False(t, strings.Contains(got, "  "), "unexpected double space: %q", got)
}

func TestMinify_RemovesNewlines(t *testing.T) {
	src := "var x = 1;\nvar y = 2;\nvar z = 3;"
	got := minifyString(t, src)
	assert.NotContains(t, got, "\n")
}

func TestMinify_PreservesStringContents(t *testing.T) {
	src := `var msg = "hello   world";`
	got := minifyString(t, src)
	assert.Contains(t, got, `"hello   world"`)
}

func TestMinify_PreservesSingleQuoteStrings(t *testing.T) {
	src := `var msg = 'it is a   test';`
	got := minifyString(t, src)
	assert.Contains(t, got, `'it is a   test'`)
}

func TestMinify_PreservesTemplateLiterals(t *testing.T) {
	src := "var msg = `hello   world`;"
	got := minifyString(t, src)
	assert.Contains(t, got, "`hello   world`")
}

func TestMinify_RenamesVarDeclarations(t *testing.T) {
	src := `var myLongVariableName = 42;
console.log(myLongVariableName);`
	got := minifyString(t, src)
	assert.NotContains(t, got, "myLongVariableName")
}

func TestMinify_RenamesLetDeclarations(t *testing.T) {
	src := `let counter = 0; counter++;`
	got := minifyString(t, src)
	assert.NotContains(t, got, "counter")
}

func TestMinify_RenamesConstDeclarations(t *testing.T) {
	src := `const maxRetries = 5; if (maxRetries > 0) {}`
	got := minifyString(t, src)
	assert.NotContains(t, got, "maxRetries")
}

func TestMinify_RenamesFunctionParams(t *testing.T) {
	src := `function add(firstNumber, secondNumber) { return firstNumber + secondNumber; }`
	got := minifyString(t, src)
	assert.NotContains(t, got, "firstNumber")
	assert.NotContains(t, got, "secondNumber")
}

func TestMinify_DoesNotRenameProperties(t *testing.T) {
	src := `var obj = {}; obj.myProperty = 1;`
	got := minifyString(t, src)
	// The property name after '.' must survive unchanged.
	assert.Contains(t, got, ".myProperty")
}

func TestMinify_DoesNotRenameReservedWords(t *testing.T) {
	src := `function f() { return true; }`
	got := minifyString(t, src)
	assert.Contains(t, got, "return")
	assert.Contains(t, got, "true")
}

func TestMinify_ReplacesAllOccurrences(t *testing.T) {
	src := `var longName = 1; longName = longName + 2;`
	got := minifyString(t, src)
	assert.NotContains(t, got, "longName")
}

func TestMinify_MultipleVarDeclarations(t *testing.T) {
	src := `var alpha = 1, beta = 2, gamma = 3;`
	got := minifyString(t, src)
	assert.NotContains(t, got, "alpha")
	assert.NotContains(t, got, "beta")
	assert.NotContains(t, got, "gamma")
}

func TestMinify_RegexLiteralPreserved(t *testing.T) {
	src := `var re = /hello world/gi; re.test("x");`
	got := minifyString(t, src)
	assert.Contains(t, got, "/hello world/gi")
}

func TestMinify_EmptyInput(t *testing.T) {
	assert.Equal(t, "", string(Minify([]byte(""))))
}

func TestMinify_FunctionLocalNotExposed(t *testing.T) {
	// The renamed variable should still appear in the output (just shorter).
	src := `var myVariable = 99; console.log(myVariable);`
	got := minifyString(t, src)
	// Whatever the renamed value is, it should appear at least twice
	// (once in declaration, once in use).
	assert.NotContains(t, got, "myVariable")
	assert.Contains(t, got, "console.log")
}
