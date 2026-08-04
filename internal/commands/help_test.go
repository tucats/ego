package commands

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// TestSplitLines checks that help text is broken into lines the same way no
// matter which of the three line ending conventions the file was written
// with.
func TestSplitLines(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "line feeds, as Unix and macOS write them",
			text: "one\ntwo\nthree",
			want: []string{"one", "two", "three"},
		},
		{
			name: "carriage return and line feed, as Windows writes them",
			text: "one\r\ntwo\r\nthree",
			want: []string{"one", "two", "three"},
		},
		{
			name: "carriage returns alone, as classic Mac OS wrote them",
			text: "one\rtwo\rthree",
			want: []string{"one", "two", "three"},
		},
		{
			name: "a file that mixes conventions",
			text: "one\r\ntwo\nthree\rfour",
			want: []string{"one", "two", "three", "four"},
		},
		{
			name: "a trailing line ending leaves a final empty line",
			text: "one\r\n",
			want: []string{"one", ""},
		},
		{
			name: "text with no line ending at all is a single line",
			text: "one",
			want: []string{"one"},
		},
		{
			name: "empty text",
			text: "",
			want: []string{""},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := splitLines(test.text)

			if len(got) != len(test.want) {
				t.Fatalf("got %d lines %q, want %d lines %q",
					len(got), got, len(test.want), test.want)
			}

			for i := range got {
				if got[i] != test.want[i] {
					t.Errorf("line %d: got %q, want %q", i, got[i], test.want[i])
				}
			}
		})
	}
}

// TestHelpTopicsFoundWithAnyLineEnding is the regression test for the bug this
// work fixes.
//
// A help topic is located by testing a line of the file for equality with
// ".topic" plus the topic name. When the help file arrived with Windows line
// endings -- which happens when Git is configured to convert text files on
// checkout, as it is by default on Windows -- every line ended with an extra
// carriage return, no line ever compared equal, and every help request
// reported "Help topic not found".
//
// The test runs the real help file through the topic printer three times, once
// converted to each convention, and requires that all three behave the same.
func TestHelpTopicsFoundWithAnyLineEnding(t *testing.T) {
	source, err := os.ReadFile("../../lib/help_en.txt")
	if err != nil {
		t.Skipf("the help file is not available in this working copy: %v", err)
	}

	// The file in the repository uses line feeds. Build the other two forms
	// from it so all three describe exactly the same help text.
	lineFeeds := string(source)
	windows := strings.ReplaceAll(lineFeeds, "\n", "\r\n")
	classicMac := strings.ReplaceAll(lineFeeds, "\n", "\r")

	conventions := []struct {
		name string
		text string
	}{
		{"line feeds", lineFeeds},
		{"carriage return and line feed", windows},
		{"carriage returns alone", classicMac},
	}

	for _, convention := range conventions {
		t.Run(convention.name, func(t *testing.T) {
			lines := splitLines(convention.text)

			// This is the exact comparison the topic printer makes, and the
			// one that failed. Checking it directly says precisely what broke,
			// rather than only that some output was missing.
			wanted := topicTag + introKey
			found := false

			for _, line := range lines {
				if line == wanted {
					found = true

					break
				}
			}

			if !found {
				t.Errorf("no line equal to %q was found; help would report the topic as missing", wanted)
			}

			output := captureStdout(t, func() {
				printTopicFromLines(introKey, lines)
			})

			if strings.Contains(output, "Help topic not found") {
				t.Errorf("the %s form of the help file reported the topic as missing", convention.name)
			}

			if strings.TrimSpace(output) == "" {
				t.Error("no help text was printed")
			}

			// Whatever was printed must not still carry carriage returns,
			// which would show up on the user's terminal as stray blank space
			// or overwritten text.
			if strings.Contains(output, "\r") {
				t.Error("the printed help text still contains carriage returns")
			}
		})
	}
}

// TestHelpKeysToleratesLineEndings checks the other half of the problem: the
// key the user typed. The console hands back the whole line, and on Windows
// that line can still end in a carriage return, so "help topics" would look
// for a topic named "topics" followed by a carriage return.
func TestHelpKeysToleratesLineEndings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		wantText string
	}{
		{
			name:  "a key ending in a line feed",
			input: []string{"help", "legal\n"},
		},
		{
			name:  "a key ending in a carriage return and line feed",
			input: []string{"help", "legal\r\n"},
		},
		{
			name:  "a key with no line ending",
			input: []string{"help", "legal"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := captureStdout(t, func() {
				help(test.input)
			})

			if strings.Contains(output, "Help topic not found") {
				t.Errorf("the topic was not found; output was:\n%s", output)
			}
		})
	}
}

// TestHelpCommand checks how a line of interactive input is recognized as a
// help request, and -- just as importantly -- what is left over afterwards.
//
// The leftover is what was broken when Ego's input came from a pipe instead of
// a console. A pipe is read in one piece, so the whole script arrived as a
// single string. Treating all of it as one help command line meant the topic
// name absorbed every following statement, and those statements were then
// thrown away without ever running.
func TestHelpCommand(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantFound bool
		wantKeys  []string
		wantRest  string
	}{
		{
			name:      "a bare help command",
			text:      "help\n",
			wantFound: true,
			wantKeys:  []string{"help"},
			wantRest:  "",
		},
		{
			name:      "a help command naming a topic",
			text:      "help legal\n",
			wantFound: true,
			wantKeys:  []string{"help", "legal"},
			wantRest:  "",
		},
		{
			name:      "a help command naming a nested topic",
			text:      "help command options\n",
			wantFound: true,
			wantKeys:  []string{"help", "command", "options"},
			wantRest:  "",
		},
		{
			name:      "a Windows line ending",
			text:      "help legal\r\n",
			wantFound: true,
			wantKeys:  []string{"help", "legal"},
			wantRest:  "",
		},
		{
			name:      "no line ending at all",
			text:      "help legal",
			wantFound: true,
			wantKeys:  []string{"help", "legal"},
			wantRest:  "",
		},
		{
			name:      "upper case is accepted",
			text:      "HELP Legal\n",
			wantFound: true,
			wantKeys:  []string{"help", "legal"},
			wantRest:  "",
		},
		{
			name:      "extra spaces between the words",
			text:      "help   command    options\n",
			wantFound: true,
			wantKeys:  []string{"help", "command", "options"},
			wantRest:  "",
		},
		{
			// This is the piped-input case: the rest of the script must come
			// back untouched so that the caller can still run it.
			name:      "a whole piped script whose first line is a help command",
			text:      "help legal\nfmt.Println(\"hello\")\nexit\n",
			wantFound: true,
			wantKeys:  []string{"help", "legal"},
			wantRest:  "fmt.Println(\"hello\")\nexit\n",
		},
		{
			name:      "a piped script with Windows line endings",
			text:      "help legal\r\nfmt.Println(\"hello\")\r\n",
			wantFound: true,
			wantKeys:  []string{"help", "legal"},
			wantRest:  "fmt.Println(\"hello\")\r\n",
		},
		{
			// An identifier that merely begins with the same four letters is
			// Ego source, not a request for documentation.
			name:      "an identifier beginning with the word help",
			text:      "helper := 42\n",
			wantFound: false,
		},
		{
			name:      "ordinary source that mentions help later on",
			text:      "fmt.Println(\"help\")\n",
			wantFound: false,
		},
		{
			name:      "empty input",
			text:      "",
			wantFound: false,
		},
		{
			name:      "a blank line",
			text:      "\n",
			wantFound: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			keys, rest, found := helpCommand(test.text)

			if found != test.wantFound {
				t.Fatalf("helpCommand(%q) found = %v, want %v", test.text, found, test.wantFound)
			}

			if !found {
				return
			}

			if strings.Join(keys, "|") != strings.Join(test.wantKeys, "|") {
				t.Errorf("keys = %q, want %q", keys, test.wantKeys)
			}

			if rest != test.wantRest {
				t.Errorf("remaining input = %q, want %q", rest, test.wantRest)
			}
		})
	}
}

// TestHelpCommandConsumesOneLineAtATime confirms that a script opening with
// several help commands has each of them recognized in turn, which is what
// lets the caller's loop work its way through them.
func TestHelpCommandConsumesOneLineAtATime(t *testing.T) {
	text := "help legal\nhelp topics\nfmt.Println(\"done\")\n"

	keys, rest, found := helpCommand(text)
	if !found || strings.Join(keys, " ") != "help legal" {
		t.Fatalf("first pass: found = %v, keys = %q", found, keys)
	}

	keys, rest, found = helpCommand(rest)
	if !found || strings.Join(keys, " ") != "help topics" {
		t.Fatalf("second pass: found = %v, keys = %q", found, keys)
	}

	if _, _, found = helpCommand(rest); found {
		t.Error("the remaining source was mistaken for a third help command")
	}

	if rest != "fmt.Println(\"done\")\n" {
		t.Errorf("remaining input = %q, want the final statement", rest)
	}
}

// captureStdout runs a function and returns everything it printed.
//
// The help code prints with fmt.Println, which writes to os.Stdout. Because
// that is a package-level variable that fmt looks up each time it is called,
// pointing it at a pipe for the duration of the call collects the output.
//
// The pipe is drained on a separate goroutine because a pipe holds only a
// limited amount of data: if nothing were reading from one end while the test
// wrote to the other, a long enough help topic would fill it and block
// forever.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("cannot create a pipe: %v", err)
	}

	previousStdout := os.Stdout
	os.Stdout = writer

	captured := make(chan string, 1)

	go func() {
		var buffer bytes.Buffer

		_, _ = io.Copy(&buffer, reader)

		captured <- buffer.String()
	}()

	f()

	// Restore stdout first, so that a failure inside f() cannot leave the
	// rest of the test run with no place to print.
	os.Stdout = previousStdout

	_ = writer.Close()

	return <-captured
}
