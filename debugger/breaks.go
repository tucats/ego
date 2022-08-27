package debugger

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/datatypes"
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

func Break(c *bytecode.Context, t *tokenizer.Tokenizer) *errors.EgoError {
	var err *errors.EgoError

	t.Advance(1)

	clear := t.IsNext("clear")

	for t.Peek(1) != tokenizer.EndOfTokens {
		switch t.Next() {
		case "when":
			text := t.GetTokens(2, len(t.Tokens), true)
			ec := compiler.New("break expression").WithTokens(tokenizer.New(text))

			bc, err := ec.Expression()
			if errors.Nil(err) {
				if clear {
					clearBreakWhen(text)

					err = nil
				} else {
					err = breakWhen(bc, text)
				}
				if !errors.Nil(err) {
					return err
				}
			}

			t.Advance(tokenizer.ToTheEnd)

		case "at":
			name := t.Next()

			if t.Peek(1) == ":" {
				t.Advance(1)
			} else {
				name = "main"

				t.Advance(-1)
			}

			line, e2 := strconv.Atoi(t.Next())
			if e2 == nil {
				if clear {
					clearBreakAtLine(name, line)

					err = nil
				} else {
					err = breakAtLine(name, line)
				}
			} else {
				err = errors.New(e2)
			}

		case "save":
			name := util.Unquote(t.Next())
			if name == "" || name == tokenizer.EndOfTokens {
				name = defaultBreakpointFilename
			}

			fmt.Printf("Saving %d breakpoints\n", len(breakPoints))

			b, e := json.MarshalIndent(breakPoints, "", "  ")
			if e == nil {
				e = ioutil.WriteFile(name, b, 0777)
			}

			err = errors.New(e)

		case "load":
			name := util.Unquote(t.Next())
			if name == "" || name == tokenizer.EndOfTokens {
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
						ec := compiler.New("break expression").WithTokens(tokenizer.New(bp.Text))

						bc, err := ec.Expression()
						if errors.Nil(err) {
							v[n].expr = bc
						} else {
							break
						}
					}
				}

				breakPoints = v

				fmt.Printf("Loaded %d breakpoints\n", len(breakPoints))
			}

			err = errors.New(e)

		default:
			err = errors.New(errors.ErrInvalidBreakClause)
		}

		if !errors.Nil(err) {
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

func breakAtLine(module string, line int) *errors.EgoError {

	for _, b := range breakPoints {
		if b.Kind == BreakAlways && b.Line == line {
			fmt.Println("Breakpoint already set")

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

	fmt.Printf("Added break %s\n", FormatBreakpoint(b))

	return nil
}

func breakWhen(expression *bytecode.ByteCode, text string) *errors.EgoError {

	for _, b := range breakPoints {
		if b.Kind == BreakValue && b.Text == text {
			fmt.Println("Breakpoint already set")

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

	fmt.Printf("Added break %s\n", FormatBreakpoint(b))

	return nil
}

func ShowBreaks() {
	if len(breakPoints) == 0 {
		fmt.Printf("No breakpoints defined\n")
	} else {
		for _, b := range breakPoints {
			fmt.Printf("break %s\n", FormatBreakpoint(b))
		}
	}
}

func FormatBreakpoint(b breakPoint) string {
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
func EvaluateBreakpoint(c *bytecode.Context) bool {
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
			if !errors.Nil(err) {
				if err.Is(errors.ErrStepOver) {
					err = nil

					ctx.StepOver(true)
				}

				if err == errors.ErrSignalDebugger {
					err = nil
				}
			}

			//fmt.Printf("Break expression status = %v\n", err)
			if errors.Nil(err) {
				if v, err := ctx.Pop(); errors.Nil(err) {
					//fmt.Printf("Break expression result = %v\n", v)
					prompt = datatypes.GetBool(v)
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
