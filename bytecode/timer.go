package bytecode

import (
	"fmt"
	"time"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
)

var TotalDuration float64

func timerByteCode(c *Context, i interface{}) error {
	mode := data.Int(i)
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

		text := ""

		if true {
			text = fmt.Sprintf("%8.6fs", elapsed.Seconds())
		} else {
			if e := elapsed.Microseconds(); e < 1000 {
				text = fmt.Sprintf("%4dµs", e)
			} else if e := elapsed.Milliseconds(); e < 1000 {
				text = fmt.Sprintf("%4dms", e)
			} else {
				text = fmt.Sprintf("%4.2f ", elapsed.Seconds())
			}
		}

		return c.push(text)

	default:
		return c.error(errors.ErrInvalidTimer).Context(mode)
	}
}
