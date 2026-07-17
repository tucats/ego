package format

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/ego/internal/language/parse"
)

// TestFormatGolden checks canonical output for representative constructs. Input
// is deliberately messy (irregular spacing/indentation) to confirm the
// formatter normalizes it.
func TestFormatGolden(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "assignment-spacing",
			src:  "x:=1+2*3",
			want: "x := 1 + 2 * 3\n",
		},
		{
			name: "call",
			src:  `fmt.Println(  "a",x )`,
			want: "fmt.Println(\"a\", x)\n",
		},
		{
			name: "if-else",
			src:  "if x>0 {y=1} else {y=2}",
			want: "if x > 0 {\n    y = 1\n} else {\n    y = 2\n}\n",
		},
		{
			name: "for-three-clause",
			src:  "for i:=0;i<10;i++ {sum+=i}",
			want: "for i := 0; i < 10; i++ {\n    sum += i\n}\n",
		},
		{
			name: "range",
			src:  "for k,v:=range m {print k,v}",
			want: "for k, v := range m {\n    print k, v\n}\n",
		},
		{
			name: "composite",
			src:  `p:=Point{X:1,Y:2}`,
			want: "p := Point{X: 1, Y: 2}\n",
		},
		{
			name: "slice-literal",
			src:  `s:=[]int{1,2,3}`,
			want: "s := []int{1, 2, 3}\n",
		},
		{
			name: "empty-slice-literal",
			src:  `s:=[]int{}`,
			want: "s := []int{}\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Source(c.src, true)
			if err != nil {
				t.Fatalf("Source(%q) error: %v", c.src, err)
			}

			if got != c.want {
				t.Errorf("Source(%q)\n  got:  %q\n  want: %q", c.src, got, c.want)
			}
		})
	}
}

// TestFormatProgram checks a whole-program layout: package/import/type/func with
// blank lines between top-level declarations.
func TestFormatProgram(t *testing.T) {
	src := `package main
import "fmt"
func main(){fmt.Println("hi")}`

	got, err := Source(src, false)
	if err != nil {
		t.Fatalf("Source error: %v", err)
	}

	want := "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"hi\")\n}\n"
	if got != want {
		t.Errorf("program format\n  got:  %q\n  want: %q", got, want)
	}
}

// TestIndentWidth checks that Options.IndentWidth controls the number of spaces
// per level, and that a zero value falls back to the 4-space default.
func TestIndentWidth(t *testing.T) {
	cases := []struct {
		width int
		want  string
	}{
		{width: 0, want: "if x {\n    y = 1\n}\n"},
		{width: 2, want: "if x {\n  y = 1\n}\n"},
		{width: 8, want: "if x {\n        y = 1\n}\n"},
	}

	for _, c := range cases {
		got, err := SourceWithOptions("if x {y=1}", true, Options{IndentWidth: c.width})
		if err != nil {
			t.Fatalf("width %d: %v", c.width, err)
		}

		if got != c.want {
			t.Errorf("width %d\n  got:  %q\n  want: %q", c.width, got, c.want)
		}
	}
}

// TestBlankLineRules checks the statement-spacing rules: a blank line before
// return/break/continue and before if (unless first in the block), a var group
// followed by a blank, a blank after a directive-with-block, and no trailing
// blank lines.
func TestBlankLineRules(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "blank-before-return",
			src:  "func f() {\nx := 1\nreturn x\n}",
			want: "func f() {\n    x := 1\n\n    return x\n}\n",
		},
		{
			name: "no-blank-first-return",
			src:  "func f() {\nreturn 1\n}",
			want: "func f() {\n    return 1\n}\n",
		},
		{
			name: "var-group-then-blank",
			src:  "func f() {\nvar x int\nvar y int\nx = 1\n}",
			want: "func f() {\n    var x int\n    var y int\n\n    x = 1\n}\n",
		},
		{
			name: "blank-before-if",
			src:  "func f() {\nx := 1\nif x > 0 {\nprint x\n}\n}",
			want: "func f() {\n    x := 1\n\n    if x > 0 {\n        print x\n    }\n}\n",
		},
		{
			name: "blank-before-break",
			src:  "func f() {\nfor {\nx := 1\nbreak\n}\n}",
			want: "func f() {\n    for {\n        x := 1\n\n        break\n    }\n}\n",
		},
		{
			name: "blanks-around-for",
			src:  "func f() {\nx := 1\nfor i := 0; i < 2; i++ {\nprint i\n}\ny := 2\n}",
			want: "func f() {\n    x := 1\n\n    for i := 0; i < 2; i++ {\n        print i\n    }\n\n    y := 2\n}\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Source(c.src, false)
			if err != nil {
				t.Fatalf("Source(%q): %v", c.src, err)
			}

			if got != c.want {
				t.Errorf("Source(%q)\n  got:  %q\n  want: %q", c.src, got, c.want)
			}
		})
	}
}

// TestNoTrailingBlankLines checks that trailing blank lines are stripped,
// leaving a single terminating newline.
func TestNoTrailingBlankLines(t *testing.T) {
	got, err := Source("x := 1\n\n\n", true)
	if err != nil {
		t.Fatalf("Source: %v", err)
	}

	if got != "x := 1\n" {
		t.Errorf("got %q, want %q", got, "x := 1\n")
	}
}

// TestCommentPreservation checks that leading, trailing, and standalone
// comments survive a format and land in sensible positions.
func TestCommentPreservation(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "leading-and-trailing",
			src:  "// count\nx := 1 // one",
			want: "// count\nx := 1  // one\n",
		},
		{
			name: "standalone-between",
			src:  "x := 1\n// separator\ny := 2",
			want: "x := 1\n// separator\ny := 2\n",
		},
		{
			name: "inside-block",
			src:  "if x {\n// inner\ny = 1 // set\n}",
			want: "if x {\n    // inner\n    y = 1  // set\n}\n",
		},
		{
			name: "block-comment",
			src:  "x := 1\n/* note */\ny := 2",
			want: "x := 1\n/* note */\ny := 2\n",
		},
		{
			// A multi-line block comment must not pick up a stray ";" from the
			// tokenizer's synthetic line endings, and its interior lines are
			// re-indented to the block level.
			name: "multiline-block-comment",
			src:  "func f() {\n/* line A\n   line B */\nx := 1\n}",
			want: "func f() {\n    /* line A\n    line B */\n    x := 1\n}\n",
		},
		{
			// A star comment aligns each "*" one column right of "/*", as gofmt
			// does, honoring the current block indentation.
			name: "star-comment-aligned",
			src:  "func f() {\n/*\n * one\n * two\n */\nx := 1\n}",
			want: "func f() {\n    /*\n     * one\n     * two\n     */\n    x := 1\n}\n",
		},
		{
			name: "trailing-in-block",
			src:  "func f() {\nreturn // done\n// last\n}",
			want: "func f() {\n    return  // done\n    // last\n}\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Source(c.src, true)
			if err != nil {
				t.Fatalf("Source(%q): %v", c.src, err)
			}

			if got != c.want {
				t.Errorf("Source(%q)\n  got:  %q\n  want: %q", c.src, got, c.want)
			}
		})
	}
}

// TestNoCommentLost verifies that every comment in the input appears in the
// formatted output for the whole corpus — the core guarantee behind in-place
// rewriting. Placement may be approximate, but nothing is dropped.
func TestNoCommentLost(t *testing.T) {
	root := repoRoot(t)

	var files []string

	for _, dir := range []string{"tests", "lib/packages"} {
		_ = filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err == nil && info != nil && !info.IsDir() && strings.HasSuffix(path, ".ego") {
				files = append(files, path)
			}

			return nil
		})
	}

	if len(files) == 0 {
		t.Skip("no corpus files found")
	}

	var lost int

	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		got, err := Source(string(src), true)
		if err != nil {
			continue
		}

		// Every "//" or "/*" that starts a comment in the source must still
		// start a comment in the output. We compare comment counts, which the
		// parser exposes via the file's Comments list.
		wantN := countComments(t, string(src))
		gotN := countComments(t, got)

		if gotN < wantN {
			lost++

			if lost <= 10 {
				rel, _ := filepath.Rel(root, path)
				t.Logf("comments lost in %s: had %d, kept %d", rel, wantN, gotN)
			}
		}
	}

	if lost > 0 {
		t.Errorf("%d files lost comments on format", lost)
	}
}

// countComments parses source as a fragment and returns the number of comments
// the parser recorded.
func countComments(t *testing.T, source string) int {
	t.Helper()

	file, err := parse.ParseStatements(source)
	if err != nil {
		return 0
	}

	return len(file.Comments)
}

// TestFormatIdempotent checks the essential property of a canonical formatter:
// formatting is a fixed point. For every corpus file, format(parse(src)) = f1,
// then format(parse(f1)) = f2, and f1 must equal f2. This holds even where f1
// differs from the original source (comments dropped, spacing normalized).
func TestFormatIdempotent(t *testing.T) {
	root := repoRoot(t)

	roots := []string{"tests", "lib/packages"}

	var files []string

	for _, dir := range roots {
		_ = filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err == nil && info != nil && !info.IsDir() && strings.HasSuffix(path, ".ego") {
				files = append(files, path)
			}

			return nil
		})
	}

	if len(files) == 0 {
		t.Skip("no corpus files found")
	}

	var notIdempotent, formatErrors int

	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		f1, err := Source(string(src), true)
		if err != nil {
			// A parse failure here would be a parser regression, caught by the
			// parser's own corpus test; skip for the formatter's purposes.
			continue
		}

		f2, err := Source(f1, true)
		if err != nil {
			formatErrors++

			rel, _ := filepath.Rel(root, path)
			t.Logf("re-parse of formatted output failed: %s: %v", rel, err)

			continue
		}

		if f1 != f2 {
			notIdempotent++

			if notIdempotent <= 10 {
				rel, _ := filepath.Rel(root, path)
				t.Logf("not idempotent: %s\n%s", rel, firstDiff(f1, f2))
			}
		}
	}

	t.Logf("idempotency: %d files, %d not idempotent, %d re-parse errors",
		len(files), notIdempotent, formatErrors)

	if notIdempotent > 0 || formatErrors > 0 {
		t.Errorf("formatter not stable: %d non-idempotent, %d re-parse errors",
			notIdempotent, formatErrors)
	}
}

// firstDiff returns a short excerpt around the first line that differs between a
// and b, for debugging non-idempotent output.
func firstDiff(a, b string) string {
	la := strings.Split(a, "\n")
	lb := strings.Split(b, "\n")

	for i := 0; i < len(la) && i < len(lb); i++ {
		if la[i] != lb[i] {
			return "  f1[" + itoa(i) + "]=" + la[i] + "\n  f2[" + itoa(i) + "]=" + lb[i]
		}
	}

	return "  (length differs)"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var buf [20]byte

	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	return string(buf[i:])
}

// repoRoot locates the repository root by walking up to the go.mod file.
func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate repo root")
		}

		dir = parent
	}
}
