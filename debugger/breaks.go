package debugger

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/tokenizer"
	"github.com/tucats/ego/util"
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

func breakCommand(c *bytecode.Context, t *tokenizer.Tokenizer) error {
	var err error

	t.Advance(1)

	clear := t.IsNext(tokenizer.ClearToken)

	for t.Peek(1) != tokenizer.EndOfTokens {
		switch t.NextText() {
		case "when":
			text := t.GetTokens(2, len(t.Tokens), true)
			ec := compiler.New("break expression").WithTokens(tokenizer.New(text, true))

			bc, err := ec.Expression()
			if err == nil {
				if clear {
					clearBreakWhen(text)

					err = nil
				} else {
					err = breakWhen(bc, text)
				}

				if err != nil {
					return err
				}
			}

			t.Advance(tokenizer.ToTheEnd)

		case "at":
			name := t.NextText()

			if t.Peek(1) == tokenizer.ColonToken {
				t.Advance(1)
			} else {
				name = defs.Main

				t.Advance(-1)
			}

			line, e2 := strconv.Atoi(t.NextText())
			if e2 == nil {
				if clear {
					clearBreakAtLine(name, line)

					err = nil
				} else {
					err = breakAtLine(name, line)
				}
			} else {
				err = errors.NewError(e2)
			}

		case "save":
			name := util.Unquote(t.NextText())
			if name == "" {
				name = defaultBreakpointFilename
			}

			ui.Say("msg.debug.save.count", map[string]interface{}{
				"count": len(breakPoints),
			})

			b, e := json.MarshalIndent(breakPoints, "", "  ")
			if e == nil {
				e = ioutil.WriteFile(name, b, 0777)
			}

			if e != nil {
				err = errors.NewError(e)
			}

		case "load":
			name := util.Unquote(t.NextText())
			if name == "" {
				name = defaultBreakpointFilename
			}

			v := []breakPoint{}

			b, e := ioutil.ReadFile(name)
			if e == nil {
				e = json.Unmarshal(b, &v)
			}

			if e == nil {
				for n, bp := range v {
					if bp.Kind == 2 {
						ec := compiler.New("break expression").
							WithTokens(tokenizer.New(bp.Text, true))

						bc, err := ec.Expression()
						if err == nil {
							v[n].expr = bc
						} else {
							break
						}
					}
				}

				breakPoints = v

				ui.Say("msg.debug.load.count", map[string]interface{}{
					"count": len(breakPoints),
				})
			}

			if e != nil {
				err = errors.NewError(e)
			}

		default:
			err = errors.ErrInvalidBreakClause
		}

		if err != nil {
			break
		}
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

func clearBreakAtLine(module string, line int) {
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

func breakAtLine(module string, line int) error {
	for _, b := range breakPoints {
		if b.Kind == BreakAlways && b.Line == line {
			ui.Say("msg.debug.break.exists")

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

	ui.Say("msg.debug.break.added", map[string]interface{}{
		"break": formatBreakpoint(b),
	})

	return nil
}

func breakWhen(expression *bytecode.ByteCode, text string) error {
	for _, b := range breakPoints {
		if b.Kind == BreakValue && b.Text == text {
			ui.Say("msg.debug.break.exists")

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

	ui.Say("msg.debug.break.added", map[string]interface{}{
		"break": formatBreakpoint(b),
	})

	return nil
}

func showBreaks() {
	if len(breakPoints) == 0 {
		ui.Say("msg.debug.no.breakpoints")
	} else {
		for _, b := range breakPoints {
			fmt.Printf("break %s\n", formatBreakpoint(b))
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

// Using the current execution state, determine if a breakpoint has
// been encountered.
func evaluationBreakpoint(c *bytecode.Context) bool {
	s := c.GetSymbols()
	msg := ""
	prompt := false

	for _, b := range breakPoints {
		switch b.Kind {
		case BreakValue:
			// If we already hit this, don't do it again on each statement. Pass.
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

			//fmt.Printf("Break expression status = %v\n", err)
			if err == nil {
				if v, err := ctx.Pop(); err == nil {
					//fmt.Printf("Break expression result = %v\n", v)
					prompt = data.Bool(v)
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
		fmt.Printf("%s\n", msg)
	}

	return prompt
}
