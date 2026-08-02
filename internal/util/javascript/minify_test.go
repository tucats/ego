package javascript

import (
	"os"
	"path/filepath"
	"regexp"
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

// The three tests below declare inside a function body on purpose. Only
// block-scoped declarations are renamed; a declaration at file scope is left
// alone because other files and inline HTML handlers may refer to it by name
// (see TestMinify_DoesNotRenameFileScope* below).

func TestMinify_RenamesVarDeclarations(t *testing.T) {
	src := `function f() {
var myLongVariableName = 42;
console.log(myLongVariableName);
}`
	got := minifyString(t, src)
	assert.NotContains(t, got, "myLongVariableName")
}

func TestMinify_RenamesLetDeclarations(t *testing.T) {
	src := `function f() { let counter = 0; counter++; }`
	got := minifyString(t, src)
	assert.NotContains(t, got, "counter")
}

func TestMinify_RenamesConstDeclarations(t *testing.T) {
	src := `function f() { const maxRetries = 5; if (maxRetries > 0) {} }`
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
	src := `function f() { var longName = 1; longName = longName + 2; }`
	got := minifyString(t, src)
	assert.NotContains(t, got, "longName")
}

func TestMinify_MultipleVarDeclarations(t *testing.T) {
	src := `function f() { var alpha = 1, beta = 2, gamma = 3; }`
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

// A BigInt literal's trailing "n" must stay attached to its digits. If the
// tokenizer treats it as a separate identifier, needsSep() inserts a space
// between the number and the "n" (both are identifier-continuation
// characters), producing "0 n" — a syntax error, since a number literal
// can't be directly followed by a bare identifier.
func TestMinify_BigIntLiteralSuffixNotSeparated(t *testing.T) {
	src := `let hi = 0n; hi = (hi << 8n) + 1n;`
	got := minifyString(t, src)
	assert.NotContains(t, got, "0 n")
	assert.Contains(t, got, "0n")
	assert.Contains(t, got, "8n")
	assert.Contains(t, got, "1n")
}

// "?." (optional chaining) is tokenized as a single two-character punctuation
// token, distinct from a plain ".". The property-read exclusion in
// renameLocals() must recognize both — otherwise a property name following
// "?." that happens to match some unrelated local variable name declared
// elsewhere in the file gets renamed too, silently corrupting the property
// access into one that always reads undefined. Here "value" is both a real
// local declared with let and a property read via optional chaining;
// only the local declaration may be renamed.
func TestMinify_OptionalChainingPropertyNotRenamed(t *testing.T) {
	src := `function f(tok){ let value = 1; return tok?.value === value; }`
	got := minifyString(t, src)
	assert.Contains(t, got, "?.value")
}

// The spread/rest operator "..." was not recognized as a single token, so it
// fell apart into three separate "." tokens. The trailing "." then made the
// rename-skip check mistake the identifier after it (e.g. "resolved" in
// "[...resolved]") for a property read and leave it unrenamed — while the
// variable's own declaration *was* renamed, since that's an unambiguous
// local. The mismatch produces a ReferenceError at runtime: the renamed
// declaration no longer has any binding under the original name.
func TestMinify_SpreadOperatorRenamesConsistently(t *testing.T) {
	src := `function f(){const resolved = new Set(); resolved.add(1); return [...resolved][0];}`
	got := minifyString(t, src)
	assert.NotContains(t, got, "resolved")
}

func TestMinify_EmptyInput(t *testing.T) {
	assert.Equal(t, "", string(Minify([]byte(""), true)))
}

func TestMinify_FunctionLocalNotExposed(t *testing.T) {
	// The renamed variable should still appear in the output (just shorter).
	src := `function f() { var myVariable = 99; console.log(myVariable); }`
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

// ── file-scope protection ────────────────────────────────────────────────────
//
// Names bound at file scope must survive minification unchanged. Two things
// depend on it: inline onclick/onchange attributes in HTML name functions the
// minifier never sees, and each asset is minified on its own, so two files
// that shared a global would otherwise be renamed on separate counters and
// disagree about the result.

func TestMinify_DoesNotRenameFileScopeDeclarations(t *testing.T) {
	src := `const SQL_KEYWORDS = new Set(['SELECT']);
let codeFormatEnabled = false;
var legacyGlobal = 1;`
	got := minifyString(t, src)
	assert.Contains(t, got, "SQL_KEYWORDS")
	assert.Contains(t, got, "codeFormatEnabled")
	assert.Contains(t, got, "legacyGlobal")
}

func TestMinify_DoesNotRenameFileScopeFunctionOrClass(t *testing.T) {
	src := `function showSettings() { return 1; }
class DsnEditor { constructor() { this.x = 1; } }`
	got := minifyString(t, src)
	assert.Contains(t, got, "function showSettings(")
	assert.Contains(t, got, "class DsnEditor")
}

func TestMinify_FileScopeNameSharedWithLocalIsNotRenamed(t *testing.T) {
	// Regression: the rename map is keyed by name alone, so a parameter that
	// happened to share a file-scope function's name used to drag that
	// function into the rename and silently break onclick="showSettings()".
	src := `function showSettings() { return 1; }
function render(showSettings) { return showSettings + 1; }`
	got := minifyString(t, src)
	assert.Contains(t, got, "function showSettings(")
}

func TestMinify_FileScopeConstSharedWithLocalIsNotRenamed(t *testing.T) {
	src := `const activeTab = 'memory';
function openTab(activeTab) { return activeTab; }
function readIt() { return activeTab; }`
	got := minifyString(t, src)
	// Both the declaration and the far-away reader must still say activeTab.
	assert.Contains(t, got, "const activeTab=")
	assert.Contains(t, got, "return activeTab;")
}

func TestMinify_SeparatelyMinifiedFilesAgreeOnSharedNames(t *testing.T) {
	// The property that makes it safe to split one script into several files:
	// each is minified independently, so any name they share must come out of
	// both unchanged.
	fileA := `const SHARED_TABLE = {};
let sharedFlag = false;
function sharedHelper(localArgument) { return localArgument + 1; }`

	fileB := `function consumer(anotherLocal) {
	if (sharedFlag) { return sharedHelper(anotherLocal); }
	return SHARED_TABLE;
}`

	gotA := minifyString(t, fileA)
	gotB := minifyString(t, fileB)

	for _, shared := range []string{"SHARED_TABLE", "sharedFlag", "sharedHelper"} {
		assert.Contains(t, gotA+gotB, shared, "shared name %q must survive in both files", shared)
	}

	// Locals are still shortened — the protection is scoped, not a blanket
	// disabling of renaming.
	assert.NotContains(t, gotA, "localArgument")
	assert.NotContains(t, gotB, "anotherLocal")
}

func TestMinify_RenamesInsideNestedBlocks(t *testing.T) {
	// Depth tracking must count any brace, not just a function body, and must
	// recover correctly after each one closes.
	src := `function outer() {
	if (true) { let innerBlockName = 1; return innerBlockName; }
}
const afterTheBlock = 2;`
	got := minifyString(t, src)
	assert.NotContains(t, got, "innerBlockName")
	assert.Contains(t, got, "afterTheBlock")
}

func TestMinify_DepthSurvivesCompactFunctionBodies(t *testing.T) {
	// No whitespace between ')' and '{'. The scan must not step over that
	// brace, or everything after it would be mistaken for file scope.
	src := `function compact(paramName){var insideName=paramName+1;return insideName;}
const stillFileScope = 3;`
	got := minifyString(t, src)
	assert.NotContains(t, got, "paramName")
	assert.NotContains(t, got, "insideName")
	assert.Contains(t, got, "stillFileScope")
}

func TestMinify_DepthSurvivesDeclarationWithoutSemicolon(t *testing.T) {
	// A declaration with no trailing ';' ends at the '}' closing its block.
	// That brace must still be counted, or the following declaration would be
	// treated as nested and renamed.
	src := `function f() { let noSemicolon = 1 }
const followsTheBrace = 2;`
	got := minifyString(t, src)
	assert.NotContains(t, got, "noSemicolon")
	assert.Contains(t, got, "followsTheBrace")
}

// TestMinify_DashboardInlineHandlersSurvive guards the concrete case that
// motivated file-scope protection. dashboard.html calls dashboard JavaScript
// functions from inline onclick/onchange attributes; the minifier only ever
// sees the .js files, so a rename there breaks the button with no error until
// someone clicks it.
//
// The dashboard's script is split across several files, each minified on its
// own exactly as the assets handler does it, and a handler may be defined in
// any of them. Reading the real files keeps this honest as they change.
func TestMinify_DashboardInlineHandlersSurvive(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "lib", "assets", "dashboard")

	sources, err := filepath.Glob(filepath.Join(dir, "dashboard-*.js"))
	if err != nil || len(sources) == 0 {
		t.Skip("dashboard JavaScript not readable from this location:", err)
	}

	html, err := os.ReadFile(filepath.Join(dir, "dashboard.html"))
	if err != nil {
		t.Skip("dashboard.html not readable from this location:", err)
	}

	// Minify each file separately — the point is that a handler defined in one
	// file must survive that file being minified with no knowledge of the rest.
	var minified strings.Builder

	for _, source := range sources {
		js, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("reading %s: %v", source, err)
		}

		minified.WriteString(string(Minify(js, true)))
		minified.WriteString("\n")
	}

	all := minified.String()
	handler := regexp.MustCompile(`on(?:click|change)="([A-Za-z_$][A-Za-z0-9_$]*)\(`)

	seen := map[string]bool{}

	for _, match := range handler.FindAllStringSubmatch(string(html), -1) {
		name := match[1]
		if seen[name] {
			continue
		}

		seen[name] = true

		declared := regexp.MustCompile(`function\s+` + regexp.QuoteMeta(name) + `\b`)
		assert.True(t, declared.MatchString(all),
			"handler %q named in dashboard.html does not survive minification", name)
	}

	// Guard the guard: if either the glob or the regex ever stops matching,
	// this test would pass vacuously.
	assert.Greater(t, len(sources), 1, "expected several dashboard script files; glob may be stale")
	assert.Greater(t, len(seen), 20, "expected to find many inline handlers; regex may be stale")
}
