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

type breakPointType int

const (
	BreakDisabled breakPointType = 0
	BreakAlways   breakPointType = iota
	BreakValue
	defaultBreakpointFilename = "ego-breakpoints.json"
)

type breakPoint struct {
	Kind   breakPointType `json:"kind"`
	Module string         `json:"module,omitempty"`
	Line   int            `json:"line,omitempty"`
	Text   string         `json:"text,omitempty"`
	expr   *bytecode.ByteCode
	hit    int
}

// This global variable maintains the list of breakpoints currently in effect.
var breakPoints = []breakPoint{}

// breakCommand handles the "break" command in the debugger. All output is
// written through sessionContext.
func breakCommand(t *tokenizer.Tokenizer, sessionContext *session) error {
	var (
		err     error
		clauses int
	)

	t.Advance(1)

	isClear := t.IsNext(tokenizer.ClearToken)

	for t.Peek(1).IsNot(tokenizer.EndOfTokens) && t.Peek(1).IsNot(tokenizer.SemicolonToken) {
		switch t.NextText() {
		case "when":
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
			name := t.NextText()

			if t.Peek(1).Is(tokenizer.ColonToken) {
				t.Advance(1)
			} else {
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
					if bp.Kind == 2 {
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

	if clauses == 0 {
		err = errors.ErrInvalidBreakClause
	}

	return err
}

func clearBreakWhen(text string) {
	for n, b := range breakPoints {
		if b.Kind == BreakValue && b.Text == text {
			if len(breakPoints) == 1 {
				breakPoints = []breakPoint{}
			} else if n == len(breakPoints)-1 {
				breakPoints = breakPoints[:n]
			} else {
				breakPoints = append(breakPoints[:n], breakPoints[n+1:]...)
			}
		}
	}
}

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
		}
	}
}

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

func showBreaks(sessionContext *session) {
	if len(breakPoints) == 0 {
		sessionContext.say("msg.debug.no.breakpoints")
	} else {
		for _, b := range breakPoints {
			sessionContext.printf("break %s\n", formatBreakpoint(b))
		}
	}
}

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

// evaluationBreakpoint uses the current execution state to determine if a
// breakpoint has been reached. All output is written through sessionContext.
func evaluationBreakpoint(c *bytecode.Context, sessionContext *session) bool {
	s := c.GetSymbols()
	msg := ""
	prompt := false

	for _, b := range breakPoints {
		switch b.Kind {
		case BreakValue:
			// If we already hit this, don't do it again on each statement.
			if b.hit > 0 {
				break
			}

			ctx := bytecode.NewContext(s, b.expr)
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
						b.hit++
					} else {
						b.hit = 0
					}
				}
			}

			msg = "Break when " + b.Text

		case BreakAlways:
			line := c.GetLine()
			module := c.GetModuleName()

			if module == b.Module && line == b.Line {
				prompt = true
				text := c.GetTokenizer().GetLine(line)
				msg = fmt.Sprintf("%s:\n\t%5d, %s", breakAt, line, text)
				b.hit++
			}
		}
	}

	if prompt {
		sessionContext.printf("%s\n", msg)
	}

	return prompt
}
