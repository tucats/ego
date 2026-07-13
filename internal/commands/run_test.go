package commands

import (
	"os"
	"testing"

	"github.com/tucats/ego/internal/cli/cli"
)

// Test_readSourceFromConsoleOrPipe_PreservesNewlines is a regression test: the
// scanner loop used to join piped stdin lines with a single space instead of
// a newline, so a "//" line comment anywhere in the input silently truncated
// everything that followed once the lines were joined onto one logical line.
// For example, piping:
//
//	// hello
//	fmt.Println("hi")
//
// produced no output at all, because the joined text became
// `// hello fmt.Println("hi") ` -- entirely a comment. Verify the assembled
// text keeps each scanned line on its own line instead.
func Test_readSourceFromConsoleOrPipe_PreservesNewlines(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("could not create pipe: %v", err)
	}

	origStdin := os.Stdin
	os.Stdin = r

	defer func() { os.Stdin = origStdin }()

	if _, err := w.WriteString("// hello\nfmt.Println(\"hi\")\n"); err != nil {
		t.Fatalf("could not write to pipe: %v", err)
	}

	w.Close()

	c := &cli.Context{}

	_, _, text, _ := readSourceFromConsoleOrPipe(false, c, false, "", "", "")

	const want = "// hello\nfmt.Println(\"hi\")\n"
	if text != want {
		t.Errorf("readSourceFromConsoleOrPipe() text = %q, want %q", text, want)
	}
}
