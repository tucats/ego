package debugger

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/egostrings"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
)

// breakPointType is an integer tag that describes what kind of trigger a
// breakpoint uses.  It is serialised to JSON when saving breakpoints to a
// file, so the underlying numeric values must remain stable.
type breakPointType int

const (
	// BreakDisabled is the zero value — a breakpoint that has been created but
	// is not currently active.  It is never added to breakPoints normally; it
	// serves as the JSON zero value for unmarshalled breakpoints that lack a
	// kind field.
	BreakDisabled breakPointType = 0

	// BreakAlways fires every time execution reaches a specific source line in
	// a specific module.  Corresponds to "break at [module:]line".
	BreakAlways breakPointType = iota

	// BreakValue fires when a user-supplied expression evaluates to true.
	// Corresponds to "break when <expression>".
	BreakValue

	// defaultBreakpointFilename is used by "break save" and "break load" when
	// the user does not supply an explicit file name.
	defaultBreakpointFilename = "ego-breakpoints.json"
)

// breakPoint holds one entry in the active breakpoint list.
//
// Kind, Module, Line, and Text are exported so they round-trip cleanly
// through JSON when the user saves/loads breakpoints.  The compiled
// expression (expr) and the hit counter (hit) are not persisted — expr is
// recompiled on load, and hit resets to zero after each save/load cycle.
type breakPoint struct {
	Kind   breakPointType `json:"kind"`
	Module string         `json:"module,omitempty"`
	Line   int            `json:"line,omitempty"`
	Text   string         `json:"text,omitempty"`

	// expr is the compiled bytecode for a BreakValue conditional expression.
	// It is reconstructed when loading breakpoints from JSON.
	expr *bytecode.ByteCode

	// hit counts how many times this breakpoint has consecutively triggered.
	// For BreakValue breakpoints it suppresses re-triggering on every step
	// while the condition remains true: once hit > 0 the breakpoint waits
	// until the condition goes false before firing again.
	hit int
}

// breakPoints is the package-level list of active breakpoints.  All access
// happens inside the single debugger goroutine, so no mutex is required.
var breakPoints = []breakPoint{}

// breakCommand parses and executes a "break ..." command entered at the
// debugger prompt.  All output is written through sessionContext.
//
// Supported sub-commands:
//
//	break at [module:]line          — set a line breakpoint
//	break when <expression>         — set a conditional breakpoint
//	break clear at [module:]line    — remove a line breakpoint
//	break clear when <expression>   — remove a conditional breakpoint
//	break save ["filename"]         — serialise breakpoints to JSON
//	break load ["filename"]         — restore breakpoints from JSON
func breakCommand(t *tokenizer.Tokenizer, sessionContext *session) error {
	var (
		err     error
		clauses int
	)

	// Skip past the leading "break" token itself.
	t.Advance(1)

	// "break clear ..." removes an existing breakpoint rather than adding one.
	isClear := t.IsNext(tokenizer.ClearToken)

	// Process one or more sub-clauses until end of the command.
	for t.Peek(1).IsNot(tokenizer.EndOfTokens) && t.Peek(1).IsNot(tokenizer.SemicolonToken) {
		switch t.NextText() {
		case "when":
			// "break when <expression>" — compile the expression to bytecode so
			// it can be evaluated on every step during execution.
			text := t.GetTokens(2, len(t.Tokens), true)
			ec := compiler.New("break expression").WithTokens(tokenizer.New(text, true))

			bc, err := ec.Expression(true)
			if err == nil {
				_, err = ec.Close()
			}

			if err == nil {
				if isClear {
					clearBreakWhen(text)
				} else {
					err = breakWhen(bc, text, sessionContext)
				}
			}

			if err != nil {
				return err
			}

			clauses++

			t.Advance(tokenizer.ToTheEnd)

		case "at":
			// "break at [module:]line" — parse the optional module name and the
			// mandatory line number.
			name := t.NextText()

			if t.Peek(1).Is(tokenizer.ColonToken) {
				// A colon follows the name, so "name" is the module identifier and the
				// token after the colon is the line number.
				t.Advance(1)
			} else {
				// No colon — "name" is actually the line number token.  Back up so
				// the next NextText() call reads it again, and use the default module.
				name = defs.Main
				t.Advance(-1)
			}

			line, e2 := egostrings.Atoi(t.NextText())
			if e2 == nil {
				if isClear {
					clearBreakAtLine(line)
				} else {
					err = breakAtLine(name, line, sessionContext)
				}
			} else {
				err = errors.New(errors.ErrInvalidInteger)
			}

			clauses++

		case "save":
			// "break save [filename]" — serialise the current breakpoint list to a
			// JSON file for later restoration with "break load".
			name := egostrings.Unquote(t.NextText())
			if name == "" {
				name = defaultBreakpointFilename
			}

			sessionContext.say("msg.debug.save.count", map[string]any{
				"count": len(breakPoints),
			})

			b, e := json.MarshalIndent(breakPoints, "", "  ")
			if e == nil {
				e = os.WriteFile(name, b, 0777)
			}

			if e != nil {
				err = errors.New(e)
			}

			clauses++

		case "load":
			// "break load [filename]" — read a previously saved JSON breakpoint
			// file and install the breakpoints.  BreakValue entries (conditional
			// breakpoints) must have their expression text recompiled because the
			// compiled bytecode is not serialised to JSON.
			name := egostrings.Unquote(t.NextText())
			if name == "" {
				name = defaultBreakpointFilename
			}

			v := []breakPoint{}

			b, e := os.ReadFile(name)
			if e == nil {
				e = json.Unmarshal(b, &v)
			}

			if e == nil {
				for n, bp := range v {
					// Only BreakValue entries carry an expression that needs
					// recompilation — BreakAlways entries store a line number instead.
					// DEBUGGER-BREAKS-2: use the named constant BreakValue here rather
					// than the raw iota value 2, so the code remains correct if the
					// breakPointType constants are ever reordered.
					if bp.Kind == BreakValue {
						ec := compiler.New("break expression").
							WithTokens(tokenizer.New(bp.Text, true))

						bc, err := ec.Expression(true)
						if err == nil {
							_, err = ec.Close()
						}

						if err == nil {
							v[n].expr = bc
						} else {
							break
						}
					}
				}

				breakPoints = v

				sessionContext.say("msg.debug.load.count", map[string]any{
					"count": len(breakPoints),
				})
			}

			if e != nil {
				err = errors.New(e)
			}

			clauses++

		default:
			err = errors.ErrInvalidBreakClause
		}

		if err != nil {
			break
		}
	}

	// A bare "break" with no sub-clause is an error.
	if clauses == 0 {
		err = errors.ErrInvalidBreakClause
	}

	return err
}

// clearBreakWhen removes the first BreakValue breakpoint whose expression
// text equals the given string.  Breakpoints are expected to be unique, so
// the function returns immediately after the first match.
//
// DEBUGGER-BREAKS-3: the previous implementation continued iterating after
// the deletion, which could skip elements due to slice-shift confusion.
// Adding an explicit return avoids this entirely.
func clearBreakWhen(text string) {
	for n, b := range breakPoints {
		if b.Kind == BreakValue && b.Text == text {
			if len(breakPoints) == 1 {
				// Removing the only element — reset to empty slice.
				breakPoints = []breakPoint{}
			} else if n == len(breakPoints)-1 {
				// Removing the last element — simply reslice.
				breakPoints = breakPoints[:n]
			} else {
				// Removing a middle element — shift the tail left.
				breakPoints = append(breakPoints[:n], breakPoints[n+1:]...)
			}

			return // stop after the first (and only expected) match
		}
	}
}

// clearBreakAtLine removes the first BreakAlways breakpoint whose line number
// matches.  Like clearBreakWhen, it returns immediately after the first match.
//
// DEBUGGER-BREAKS-3: same fix as clearBreakWhen — explicit return after
// deletion prevents stale iteration over the shifted backing array.
func clearBreakAtLine(line int) {
	for n, b := range breakPoints {
		if b.Kind == BreakAlways && b.Line == line {
			if len(breakPoints) == 1 {
				breakPoints = []breakPoint{}
			} else if n == len(breakPoints)-1 {
				breakPoints = breakPoints[:n]
			} else {
				breakPoints = append(breakPoints[:n], breakPoints[n+1:]...)
			}

			return // stop after the first (and only expected) match
		}
	}
}

// breakAtLine adds a BreakAlways breakpoint for the given module and line.
// If an identical breakpoint already exists it reports that to the user and
// returns without adding a duplicate.
func breakAtLine(module string, line int, sessionContext *session) error {
	for _, b := range breakPoints {
		if b.Kind == BreakAlways && b.Line == line {
			sessionContext.say("msg.debug.break.exists")

			return nil
		}
	}

	b := breakPoint{
		Module: module,
		Line:   line,
		hit:    0,
		Kind:   BreakAlways,
	}
	breakPoints = append(breakPoints, b)

	sessionContext.say("msg.debug.break.added", map[string]any{
		"break": formatBreakpoint(b),
	})

	return nil
}

// breakWhen adds a BreakValue breakpoint for the given compiled expression.
// The text parameter is the original source text of the expression; it is
// stored so the breakpoint can be serialised and redisplayed.
func breakWhen(expression *bytecode.ByteCode, text string, sessionContext *session) error {
	for _, b := range breakPoints {
		if b.Kind == BreakValue && b.Text == text {
			sessionContext.say("msg.debug.break.exists")

			return nil
		}
	}

	b := breakPoint{
		Module: "expression",
		hit:    0,
		Kind:   BreakValue,
		expr:   expression,
		Text:   text,
	}
	breakPoints = append(breakPoints, b)

	sessionContext.say("msg.debug.break.added", map[string]any{
		"break": formatBreakpoint(b),
	})

	return nil
}

// showBreaks prints all active breakpoints to the session writer, or a
// "no breakpoints" message if the list is empty.
func showBreaks(sessionContext *session) {
	if len(breakPoints) == 0 {
		sessionContext.say("msg.debug.no.breakpoints")
	} else {
		for _, b := range breakPoints {
			sessionContext.printf("break %s\n", formatBreakpoint(b))
		}
	}
}

// formatBreakpoint returns a human-readable description of a single breakpoint,
// suitable for display in "show breaks" output and confirmation messages.
func formatBreakpoint(b breakPoint) string {
	switch b.Kind {
	case BreakAlways:
		return fmt.Sprintf("at %s:%d", b.Module, b.Line)

	case BreakValue:
		return fmt.Sprintf("when %s", b.Text)

	default:
		return fmt.Sprintf("(undefined) %v", b)
	}
}

// evaluationBreakpoint is called on every debugger stop to decide whether
// any active breakpoint condition is satisfied.  It returns true when at
// least one breakpoint fires, which causes the debugger to enter the
// interactive prompt.
//
// DEBUGGER-BREAKS-1: the previous implementation iterated with
//
//	for _, b := range breakPoints
//
// which creates a VALUE COPY of each element.  Assigning to b.hit inside the
// loop modified the copy only — the real slice element was never updated.
// This made the hit-count suppression logic completely ineffective: the check
// "if b.hit > 0 { break }" always saw hit == 0, so conditional breakpoints
// fired on every step where the expression was true rather than just once
// per "becoming-true" edge.
//
// The fix uses index-based access (for n := range breakPoints) and reads/
// writes breakPoints[n] directly so hit-count changes persist across calls.
func evaluationBreakpoint(c *bytecode.Context, sessionContext *session) bool {
	s := c.GetSymbols()
	msg := ""
	prompt := false

	for n := range breakPoints {
		switch breakPoints[n].Kind {
		case BreakValue:
			// hit > 0 means this conditional breakpoint already fired on the
			// last step and the condition has not yet gone false.  Skip it to
			// avoid stopping on every single subsequent step while the
			// expression remains true.
			if breakPoints[n].hit > 0 {
				break
			}

			// Run the compiled expression in a fresh child context that shares
			// the program's current symbol table so the expression can reference
			// any variable the program can see.
			ctx := bytecode.NewContext(s, breakPoints[n].expr)
			ctx.SetDebug(false)

			err := ctx.Run()
			if err != nil {
				if errors.Equals(err, errors.ErrStepOver) {
					err = nil
					ctx.StepOver(true)
				}

				if err == errors.ErrSignalDebugger {
					err = nil
				}
			}

			if err == nil {
				if v, err := ctx.Pop(); err == nil {
					prompt, _ = data.Bool(v)
					if prompt {
						// Condition became true — record the hit so it is
						// suppressed on the next step (until condition goes
						// false again).
						breakPoints[n].hit++
					} else {
						// Condition is false — reset the hit counter so the
						// breakpoint can fire again if the condition becomes
						// true in the future.
						breakPoints[n].hit = 0
					}
				}
			}

			msg = "Break when " + breakPoints[n].Text

		case BreakAlways:
			// Fire when the current context is at the exact module+line stored
			// in the breakpoint.
			line := c.GetLine()
			module := c.GetModuleName()

			if module == breakPoints[n].Module && line == breakPoints[n].Line {
				prompt = true
				text := c.GetTokenizer().GetLine(line)
				msg = fmt.Sprintf("%s:\n\t%5d, %s", breakAt, line, text)
				breakPoints[n].hit++ // track cumulative hit count for diagnostics
			}
		}
	}

	if prompt {
		sessionContext.printf("%s\n", msg)
	}

	return prompt
}
