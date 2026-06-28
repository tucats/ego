package javascript

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// helper: minify src and return as string.
func minifyString(t *testing.T, src string) string {
	t.Helper()

	return string(Minify([]byte(src), true))
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
	assert.Equal(t, "", string(Minify([]byte(""), true)))
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

func TestMinify_PreservesExplicitObjectLiteralKeys(t *testing.T) {
	// Explicit property keys ({key: value}) must not be renamed even when a
	// local variable shares the name.
	src := `function login(body) { return JSON.stringify({body: body}); }`
	got := minifyString(t, src)
	// The property key "body:" must be preserved verbatim.
	assert.Contains(t, got, "body:")
	// The parameter (as a value, not a key) should have been renamed.
	assert.NotContains(t, got, "body)")
}

func TestMinify_ExpandsShorthandProperties(t *testing.T) {
	// ES6 shorthand property notation: {username} is sugar for {username: username}.
	// The minifier must expand shorthand properties when the variable is renamed,
	// keeping the original identifier as the property key so that consumers
	// (e.g. a server expecting {"username":...}) receive the correct field name.
	src := `function login(username, password) { return JSON.stringify({username, password, source: 'Dashboard'}); }`
	got := minifyString(t, src)
	// The property keys must be the original names.
	assert.Contains(t, got, "username:")
	assert.Contains(t, got, "password:")
	// The source key (not a renamed local) must pass through unchanged.
	assert.Contains(t, got, "source:")
	// The parameters themselves should have been renamed.
	assert.NotContains(t, got, ",username,")
	assert.NotContains(t, got, ",password,")
}

func TestMinify_PreservesFunctionDeclarationNames(t *testing.T) {
	// Named function declarations may be called from HTML onclick/onchange
	// attributes (e.g. onclick="openTab('memory')"). The minifier must not
	// rename them, because it cannot see or update those HTML references.
	src := `function openTab(tabId) { return tabId; }
function flushCaches() { return true; }
function showDetail(name) { return name; }`
	got := minifyString(t, src)
	assert.Contains(t, got, "openTab")
	assert.Contains(t, got, "flushCaches")
	assert.Contains(t, got, "showDetail")
	// Parameters are still renamed (they are truly local).
	assert.NotContains(t, got, "tabId")
	assert.NotContains(t, got, "name")
}

func TestMinify_DoesNotExpandFunctionParams(t *testing.T) {
	// Function parameters share the (a, b) grammar with object shorthand but
	// must not be expanded. Expanding 'value' to 'value:short' inside a param
	// list produces invalid syntax ('value:m1' looks like a TypeScript annotation).
	src := `function setCookie(name, value, maxAgeSeconds) {
		let cookie = encodeURIComponent(name) + '=' + encodeURIComponent(value);
		if (maxAgeSeconds) cookie += '; max-age=' + maxAgeSeconds;
		document.cookie = cookie;
	}`
	got := minifyString(t, src)
	// No parameter must be emitted as 'param:short' — that is invalid JS.
	assert.NotContains(t, got, "name:")
	assert.NotContains(t, got, "value:")
	assert.NotContains(t, got, "maxAgeSeconds:")
}

func TestMinify_ExpandsShorthandDestructuring(t *testing.T) {
	// Destructuring also uses shorthand: const {username} = obj means
	// "bind property 'username' to local 'username'". After rename it must
	// become const {username: short} = obj, not const {short} = obj (which
	// would try to bind the property named 'short').
	src := `function process(obj) { const {username, password} = obj; return username + password; }`
	got := minifyString(t, src)
	// Property keys in the destructuring pattern must be preserved.
	assert.Contains(t, got, "username:")
	assert.Contains(t, got, "password:")
}
