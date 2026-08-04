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
	session := &runSession{}

	if err := session.readSourceFromConsole(c); err != nil {
		t.Fatalf("readSourceFromConsole() returned an error: %v", err)
	}

	const want = "// hello\nfmt.Println(\"hi\")\n"
	if session.text != want {
		t.Errorf("readSourceFromConsole() text = %q, want %q", session.text, want)
	}

	// Reading from a pipe means the whole program arrived at once, so the run
	// loop must not go back for more.
	if !session.wasCommandLine {
		t.Error("piped input should be treated as a program supplied all at once")
	}

	if session.mainName != stdinSourceName {
		t.Errorf("source name = %q, want %q", session.mainName, stdinSourceName)
	}
}
