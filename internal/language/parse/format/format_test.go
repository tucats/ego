package format

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
