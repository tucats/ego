package compiler

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/language/symbols"
)

// The tests in this file pin down the line number that appears in a compile
// error, which is the subject of docs/issues/REPL-1.md. Three separate faults
// in this package were fixed there, and each one is checked below:
//
//   - compileError named the token *before* the offending one, so an error
//     about the first token on a line was reported against the previous line.
//   - Clone did not carry the "@line" offset across, so any error raised while
//     compiling an expression -- which is nearly all of them -- ignored the
//     directive completely.
//   - The offset the "@line" directive computed assumed the directive was the
//     first line of the text, so a second "@line" further down the text was
//     wrong by however far down it was.

// compileForLineNumber compiles the given text and returns the error it
// produced, having first made sure that an unknown symbol really is an error.
//
// That last part matters: whether a name the compiler has never seen is
// reported at all is controlled by a setting, and if it is switched off the
// mistake is left to be discovered at run time instead. These tests are about
// where a *compile* error points, so the setting is forced on and restored
// afterwards.
func compileForLineNumber(t *testing.T, text string) error {
	t.Helper()

	previous := settings.Get(defs.UnknownVarSetting)

	defer settings.SetDefault(defs.UnknownVarSetting, previous)
	settings.SetDefault(defs.UnknownVarSetting, defs.True)

	symbolTable := symbols.NewSymbolTable("line number test")

	err := RunString("test", symbolTable, text)
	if err == nil {
		t.Fatalf("expected an error compiling:\n%s", text)
	}

	return err
}

// assertReportsLine checks that an error message names the expected line.
//
// The message is matched rather than picked apart because a location is
// written several different ways: "at line 7:1" when only a line and column
// are known, "at test(line 4)" when the compilation unit has a name and the
// fault was found while running, and "at b.ego(line 5:1)" for a named source
// file. All of them contain the words "line N", so the pattern below looks for
// that with nothing but a non-digit after it -- which is what keeps line 1 from
// matching a report of line 12.
func assertReportsLine(t *testing.T, err error, want int) {
	t.Helper()

	pattern := regexp.MustCompile(`line ` + strconv.Itoa(want) + `\D`)
	if !pattern.MatchString(err.Error()) {
		t.Errorf("error should name line %d, but reads: %v", want, err)
	}
}

// TestErrorNamesTheOffendingToken checks that an error about the first token
// on a line is reported against that line, not the one before it.
//
// This is the plain-script case from REPL-1. The undefined name is the first
// thing on line 5; the old code named the last token of line 3 instead,
// because it asked the tokenizer for the token behind the one it was looking
// at.
func TestErrorNamesTheOffendingToken(t *testing.T) {
	source := "x := 1\n" + // line 1
		"fmt.Println(x)\n" + // line 2
		"\n" + // line 3
		"\n" + // line 4
		"undefined_thing()\n" // line 5

	err := compileForLineNumber(t, source)

	assertReportsLine(t, err, 5)
}

// TestLineDirectiveRenumbersFollowingLines checks the basic promise of the
// "@line" directive: the line after it is the number it names.
func TestLineDirectiveRenumbersFollowingLines(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{
			// The shape the console prompt uses: the directive on its own
			// line, then the one statement the user typed.
			name: "the first console statement",
			text: "@line 1;\nundefined_thing()\n",
			want: 1,
		},
		{
			name: "the fourth console statement",
			text: "@line 4;\nundefined_thing()\n",
			want: 4,
		},
		{
			// Renumbering to something far away proves the number really comes
			// from the directive rather than from counting the text.
			name: "a directive naming a distant line",
			text: "@line 90;\nundefined_thing()\n",
			want: 90,
		},
		{
			// Several lines after the directive, each one still counts.
			name: "the third line after the directive",
			text: "@line 10;\nx := 1\nfmt.Println(x)\nundefined_thing()\n",
			want: 12,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertReportsLine(t, compileForLineNumber(t, test.text), test.want)
		})
	}
}

// TestLineDirectiveIsRelativeToItsOwnPosition checks a directive that is not
// the first line of the text.
//
// This is the project case: "ego run --project" joins every source file in a
// directory into one piece of text, putting an "@line 1" ahead of each. The
// second file's directive is a long way down the joined text, and the old
// offset arithmetic assumed every directive sat at the very top -- so the
// second file's errors were reported with line numbers that carried on from
// the end of the first file.
func TestLineDirectiveIsRelativeToItsOwnPosition(t *testing.T) {
	// Stand in for two joined source files. The mistake is on the third line
	// of the second one.
	source := "@line 1\n" +
		"first := 1\n" +
		"fmt.Println(first)\n" +
		"\n" +
		"\n" +
		"\n" +
		"@line 1\n" +
		"second := 2\n" +
		"fmt.Println(second)\n" +
		"undefined_thing()\n"

	err := compileForLineNumber(t, source)

	assertReportsLine(t, err, 3)
}

// TestLineNumberSurvivesExpressionCompilation checks that the "@line" offset
// is honored by errors raised from inside an expression.
//
// Expressions are compiled by a clone of the compiler, and the clone used to
// start with an offset of zero however the original had been renumbered. Since
// an undefined name is found while compiling an expression, that alone was
// enough to make every renumbered error report its raw position in the text.
func TestLineNumberSurvivesExpressionCompilation(t *testing.T) {
	// The undefined name appears part-way through an expression rather than at
	// the start of a statement, so it can only be reached by the clone.
	source := "@line 50;\ntotal := 1 + undefined_thing\n"

	err := compileForLineNumber(t, source)

	assertReportsLine(t, err, 50)
}
