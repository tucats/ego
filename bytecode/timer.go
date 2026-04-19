package bytecode

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

var TotalDuration float64

func timerByteCode(c *Context, i any) error {
	mode, err := data.Int(i)
	if err != nil {
		return c.runtimeError(err)
	}

	switch mode {
	case 0:
		t := time.Now()
		c.timerStack = append(c.timerStack, t)

		return nil

	case 1:
		timerStack := len(c.timerStack)
		if timerStack == 0 {
			return c.push("<none>")
		}

		t := c.timerStack[timerStack-1]
		c.timerStack = c.timerStack[:timerStack-1]
		elapsed := time.Since(t)
		TotalDuration += elapsed.Seconds()
		text := fmt.Sprintf("%16s", formatElapsed(elapsed))

		return c.push(text)

	default:
		return c.runtimeError(errors.ErrInvalidTimer).Context(mode)
	}
}

func formatElapsed(elapsed time.Duration) string {
	text := elapsed.String()
	result := strings.Builder{}
	dot := false
	padded := false
	digits := 0
	suffixLen := 0

	for _, ch := range text {
		if dot {
			if unicode.IsDigit(ch) {
				digits++

				if digits > 3 {
					continue
				}
			} else {
				for digits < 3 {
					result.WriteRune('0')

					digits++
				}

				padded = true
			}
		} else if ch != '.' && !unicode.IsDigit(ch) && !padded {
			result.WriteString(".000")

			padded = true
		}

		if ch == '.' {
			dot = true
		} else if !unicode.IsDigit(ch) {
			suffixLen++
		}

		result.WriteRune(ch)
	}

	if suffixLen == 1 {
		result.WriteRune(' ')
	}

	return result.String()
}
