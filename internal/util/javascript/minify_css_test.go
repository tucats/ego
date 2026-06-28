package javascript

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func minifyCSSString(t *testing.T, src string) string {
	t.Helper()

	return string(MinifyCSS([]byte(src)))
}

func TestMinifyCSS_RemovesBlockComments(t *testing.T) {
	src := `/* global reset */
body { margin: 0; }`
	got := minifyCSSString(t, src)
	assert.NotContains(t, got, "/*")
	assert.NotContains(t, got, "global reset")
	assert.Contains(t, got, "body")
}

func TestMinifyCSS_CollapsesWhitespace(t *testing.T) {
	src := "body   {   margin:   0;   }"
	got := minifyCSSString(t, src)
	assert.NotContains(t, got, "  ")
}

func TestMinifyCSS_RemovesNewlines(t *testing.T) {
	src := "body {\n  color: red;\n  margin: 0;\n}"
	got := minifyCSSString(t, src)
	assert.NotContains(t, got, "\n")
}

func TestMinifyCSS_SpacesAroundBraces(t *testing.T) {
	src := "a { color: red; }"
	got := minifyCSSString(t, src)
	assert.Contains(t, got, "a{")
	assert.Contains(t, got, "{color")
	assert.Contains(t, got, "red}")
}

func TestMinifyCSS_SpacesAroundSemicolon(t *testing.T) {
	src := "a { color: red ; margin: 0 ; }"
	got := minifyCSSString(t, src)
	assert.NotContains(t, got, " ;")
	assert.NotContains(t, got, "; ")
}

func TestMinifyCSS_SpacesAroundComma(t *testing.T) {
	src := "a , b , c { color: red; }"
	got := minifyCSSString(t, src)
	assert.Contains(t, got, "a,b,c")
}

func TestMinifyCSS_RemovesSpaceAfterColon(t *testing.T) {
	src := "a { color: red; font-size: 14px; }"
	got := minifyCSSString(t, src)
	assert.Contains(t, got, "color:red")
	assert.Contains(t, got, "font-size:14px")
}

func TestMinifyCSS_PreservesSpaceBeforeColon(t *testing.T) {
	// "a :hover" selects a hover element that is a descendant of a.
	// "a:hover" selects an <a> element in hover state — different semantics.
	// The space before ':' must NOT be stripped.
	src := "a :hover { color: blue; }"
	got := minifyCSSString(t, src)
	assert.Contains(t, got, "a :hover")
}

func TestMinifyCSS_RemovesTrailingSemicolon(t *testing.T) {
	// The final semicolon in a rule block is redundant and should be removed.
	src := "a { color: red; margin: 0; }"
	got := minifyCSSString(t, src)
	assert.Contains(t, got, "margin:0}")
	assert.NotContains(t, got, "0;}")
}

func TestMinifyCSS_CollapsesConsecutiveSemicolons(t *testing.T) {
	src := "a { color: red;; margin: 0; }"
	got := minifyCSSString(t, src)
	assert.NotContains(t, got, ";;")
}

func TestMinifyCSS_PreservesQuotedStrings(t *testing.T) {
	src := `a { content: "  hello   world  "; }`
	got := minifyCSSString(t, src)
	assert.Contains(t, got, `"  hello   world  "`)
}

func TestMinifyCSS_PreservesSingleQuotedStrings(t *testing.T) {
	src := `a { font-family: 'Times New Roman', serif; }`
	got := minifyCSSString(t, src)
	assert.Contains(t, got, `'Times New Roman'`)
}

func TestMinifyCSS_MultiValueProperty(t *testing.T) {
	// Space-separated values like margin shorthand must keep their spaces.
	src := "a { margin: 0 10px 0 10px; }"
	got := minifyCSSString(t, src)
	assert.Contains(t, got, "margin:0 10px 0 10px")
}

func TestMinifyCSS_SpacesAroundCombinator(t *testing.T) {
	src := "a > b { color: red; }"
	got := minifyCSSString(t, src)
	assert.Contains(t, got, "a>b")
}

func TestMinifyCSS_EmptyInput(t *testing.T) {
	assert.Equal(t, "", string(MinifyCSS([]byte(""))))
}

func TestMinifyCSS_InlineComment(t *testing.T) {
	src := "a { color: /* primary */ red; }"
	got := minifyCSSString(t, src)
	assert.NotContains(t, got, "/*")
	assert.NotContains(t, got, "primary")
	assert.Contains(t, got, "color:red")
}

func TestMinifyCSS_MediaQuery(t *testing.T) {
	src := "@media (max-width: 600px) { body { font-size: 14px; } }"
	got := minifyCSSString(t, src)
	assert.Contains(t, got, "max-width:600px")
	assert.Contains(t, got, "font-size:14px")
	assert.NotContains(t, got, "\n")
}
