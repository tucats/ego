package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNormalizeLineEndings checks that a script written on any platform is
// presented to the compiler with the line endings it expects.
func TestNormalizeLineEndings(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"line feeds are left alone", "a\nb\n", "a\nb\n"},
		{"windows endings become line feeds", "a\r\nb\r\n", "a\nb\n"},
		{"classic mac endings become line feeds", "a\rb\r", "a\nb\n"},
		{"a file that mixes conventions", "a\r\nb\nc\r", "a\nb\nc\n"},
		{"text with no line endings", "abc", "abc"},
		{"empty text", "", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeLineEndings(test.text); got != test.want {
				t.Errorf("normalizeLineEndings(%q) = %q, want %q", test.text, got, test.want)
			}
		})
	}
}

// TestReadAllStdin covers reading a piped program.
//
// The long-line case is the regression test that matters most. This code used
// to read stdin with a bufio.Scanner, which refuses to return a line longer
// than 64KB. On hitting that limit it printed a warning and carried on with
// however much it had read, so a script containing one very long line was
// silently cut short and the surviving fragment was executed, with a
// successful exit status.
func TestReadAllStdin(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "an ordinary script",
			input: "fmt.Println(\"hi\")\n",
			want:  "fmt.Println(\"hi\")\n",
		},
		{
			name:  "comments stay on their own lines",
			input: "// hello\nfmt.Println(\"hi\")\n",
			want:  "// hello\nfmt.Println(\"hi\")\n",
		},
		{
			name:  "windows line endings are normalized",
			input: "// hello\r\nfmt.Println(\"hi\")\r\n",
			want:  "// hello\nfmt.Println(\"hi\")\n",
		},
		{
			name:  "a missing final line ending is supplied",
			input: "fmt.Println(\"hi\")",
			want:  "fmt.Println(\"hi\")\n",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := readStdinForTest(t, test.input)

			if got != test.want {
				t.Errorf("readAllStdin() = %q, want %q", got, test.want)
			}
		})
	}

	t.Run("a line longer than the old 64KB scanner limit survives intact", func(t *testing.T) {
		// 70KB comfortably exceeds bufio.Scanner's default maximum token size
		// of 64KB, which is what used to truncate the script.
		long := strings.Repeat("x", 70000)
		input := "fmt.Println(\"start\")\n// " + long + "\nfmt.Println(\"end\")\n"

		got := readStdinForTest(t, input)

		if got != input {
			t.Errorf("a %d byte script came back as %d bytes", len(input), len(got))
		}

		if !strings.Contains(got, "fmt.Println(\"end\")") {
			t.Error("the text after the long line was lost, so the script would run only partway")
		}
	})
}

// readStdinForTest points os.Stdin at a pipe holding the given text, calls
// readAllStdin, and restores os.Stdin afterwards.
//
// The write happens on a separate goroutine because a pipe holds only a
// limited amount of data: writing 70KB into one while nothing is reading the
// other end would fill it and block forever.
func readStdinForTest(t *testing.T, input string) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("could not create a pipe: %v", err)
	}

	previousStdin := os.Stdin
	os.Stdin = r

	defer func() { os.Stdin = previousStdin }()

	go func() {
		_, _ = w.WriteString(input)

		_ = w.Close()
	}()

	text, err := readAllStdin()
	if err != nil {
		t.Fatalf("readAllStdin() returned an error: %v", err)
	}

	return text
}

// TestLoadFile covers reading a single source file, including the convenience
// of supplying the ".ego" extension when the name given does not exist.
func TestLoadFile(t *testing.T) {
	dir := t.TempDir()

	program := "package main\nfunc main() {}\n"

	named := filepath.Join(dir, "named.ego")
	if err := os.WriteFile(named, []byte(program), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("a file named exactly", func(t *testing.T) {
		text, isProject, mainName, err := loadFile(named, "main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if isProject {
			t.Error("a single file should not be reported as a project")
		}

		if mainName != named {
			t.Errorf("source name = %q, want %q", mainName, named)
		}

		if !strings.HasPrefix(text, program) {
			t.Error("the file contents were not returned")
		}

		// The entry point directive is what actually causes main to be called.
		if !strings.Contains(text, "@entrypoint main") {
			t.Error("no entry point directive was appended")
		}
	})

	t.Run("the .ego extension is supplied when the name given does not exist", func(t *testing.T) {
		withoutExtension := strings.TrimSuffix(named, ".ego")

		text, _, _, err := loadFile(withoutExtension, "main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.HasPrefix(text, program) {
			t.Error("the file contents were not returned")
		}
	})

	t.Run("a custom entry point is honored", func(t *testing.T) {
		text, _, _, err := loadFile(named, "other")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(text, "@entrypoint other") {
			t.Error("the requested entry point was not used")
		}
	})

	t.Run("a missing file is reported using the name the user typed", func(t *testing.T) {
		missing := filepath.Join(dir, "not-here")

		_, _, _, err := loadFile(missing, "main")
		if err == nil {
			t.Fatal("expected an error for a file that does not exist")
		}

		// Reporting "not-here.ego does not exist" when the user asked for
		// "not-here" would be more confusing than helpful.
		if strings.Contains(err.Error(), "not-here.ego") {
			t.Errorf("the error names the extended file name rather than the one given: %v", err)
		}
	})
}

// TestLoadProject covers reading a directory of source files, and in
// particular that its failures are reported as errors.
//
// Both failure paths used to print a message and call os.Exit(2). Exiting from
// there skipped every deferred cleanup the caller had registered -- most
// visibly the one that finishes writing a --pprof profile, which was left as a
// zero-byte, unreadable file.
func TestLoadProject(t *testing.T) {
	t.Run("every source file in the directory is included", func(t *testing.T) {
		dir := t.TempDir()

		for name, body := range map[string]string{
			"a.ego":       "func alpha() {}\n",
			"b.ego":       "func beta() {}\n",
			"notego.txt":  "this is not Ego source\n",
			"noextension": "neither is this\n",
		} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
		}

		text, isProject, mainName, err := loadProject(dir, "main")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !isProject {
			t.Error("a directory should be reported as a project")
		}

		if !strings.HasSuffix(mainName, string(filepath.Separator)) {
			t.Errorf("source name %q should end with a path separator to read as a directory", mainName)
		}

		for _, want := range []string{"func alpha()", "func beta()", "@entrypoint main"} {
			if !strings.Contains(text, want) {
				t.Errorf("the joined source is missing %q", want)
			}
		}

		for _, unwanted := range []string{"this is not Ego source", "neither is this"} {
			if strings.Contains(text, unwanted) {
				t.Errorf("a file that is not Ego source was included: %q", unwanted)
			}
		}
	})

	t.Run("a directory with no source files is an error, not an exit", func(t *testing.T) {
		if _, _, _, err := loadProject(t.TempDir(), "main"); err == nil {
			t.Error("expected an error for a directory containing no Ego source")
		}
	})

	t.Run("a directory that cannot be read is an error, not an exit", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "not-here")

		if _, _, _, err := loadProject(missing, "main"); err == nil {
			t.Error("expected an error for a directory that does not exist")
		}
	})
}

// TestDeclaresFunction checks how a piped program is recognized as a complete
// program rather than a few loose statements.
//
// The cases that matter are the ones where the words look right but declare
// nothing. Searching the text for "func main(" would accept all of them;
// scanning tokens does not, because the tokenizer discards comments entirely
// and returns a whole string literal as a single token.
func TestDeclaresFunction(t *testing.T) {
	tests := []struct {
		name   string
		source string
		lookup string
		want   bool
	}{
		{"a plain declaration", "func main() {}\n", "main", true},
		{"a declaration after a package statement", "package main\n\nfunc main() {\n    fmt.Println(\"hi\")\n}\n", "main", true},
		{"a declaration with parameters", "func main(argc int) {}\n", "main", true},
		{"a different name is found when asked for", "func other() {}\n", "other", true},
		{"loose statements declare nothing", "fmt.Println(1 + 2)\n", "main", false},
		{"a mention in a line comment is not a declaration", "// func main() is not written yet\nfmt.Println(1)\n", "main", false},
		{"a mention in a block comment is not a declaration", "/* func main() {} */\nfmt.Println(1)\n", "main", false},
		{"a mention inside a string is not a declaration", "message := \"call func main() to start\"\n", "main", false},
		{"a name that merely starts the same way is not a match", "func mainLoop() {}\n", "main", false},
		{"a function declared under another name", "func other() {}\n", "main", false},
		{"empty source", "", "main", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := declaresFunction(test.source, test.lookup); got != test.want {
				t.Errorf("declaresFunction(%q, %q) = %v, want %v",
					test.source, test.lookup, got, test.want)
			}
		})
	}
}

// TestEntryPointForPipedSource checks the decision about whether a piped
// program should have its entry point called.
//
// A complete program piped in used to be compiled and then discarded, which
// produced no output and a successful exit status. Loose statements, by
// contrast, are meant to run as they stand, so nothing should be called for
// them.
func TestEntryPointForPipedSource(t *testing.T) {
	const program = "package main\nfunc main() {\n    fmt.Println(\"hi\")\n}\n"

	const statements = "fmt.Println(1 + 2)\n"

	tests := []struct {
		name       string
		source     string
		entryPoint string
		given      bool
		wantCall   string // the directive expected at the end, or "" for none
	}{
		{
			name:       "a complete program is called without being asked",
			source:     program,
			entryPoint: "main",
			wantCall:   "\n@entrypoint main",
		},
		{
			name:       "loose statements are left to run as they are",
			source:     statements,
			entryPoint: "main",
			wantCall:   "",
		},
		{
			name:       "naming the entry point explicitly also calls it",
			source:     program,
			entryPoint: "main",
			given:      true,
			wantCall:   "\n@entrypoint main",
		},
		{
			// The user said plainly that they want this called, so the
			// directive is emitted and a missing function becomes an error
			// they can see rather than silence.
			name:       "an explicitly named entry point is called even if absent",
			source:     statements,
			entryPoint: "main",
			given:      true,
			wantCall:   "\n@entrypoint main",
		},
		{
			name:       "a custom entry point is detected on its own name",
			source:     "func other() {}\n",
			entryPoint: "other",
			wantCall:   "\n@entrypoint other",
		},
		{
			// Only the named entry point counts: a program with a main, but a
			// request for something else, has not been asked for main.
			name:       "a main function does not satisfy a request for another name",
			source:     program,
			entryPoint: "other",
			wantCall:   "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session := &runSession{
				entryPoint:      test.entryPoint,
				entryPointGiven: test.given,
			}

			if got := session.entryPointForPipedSource(test.source); got != test.source+test.wantCall {
				t.Errorf("entryPointForPipedSource() = %q, want %q",
					got, test.source+test.wantCall)
			}
		})
	}
}
