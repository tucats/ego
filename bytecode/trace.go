package bytecode

import (
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
)

// Trace instruction traces a single instruction in the current context. It formats a TRACE
// log line that contains the instruction, arguments, and current stack information. If the
// trace logger is not active, no work is done.
func traceInstruction(c *Context, i instruction) {
	if !ui.IsActive(ui.TraceLogger) {
		return
	}

	instruction, operand := FormatInstruction(i)

	// If we're doing JSON logging, we can do this better by formatting a custom message
	if ui.LogFormat != ui.TextFormat {
		stackSlice := make([]string, 0)
		count := 0

		for i := c.stackPointer - 1; i >= 0; i-- {
			item := c.stack[i]
			text := data.Format(item)

			if _, isString := item.(string); !isString {
				text = strings.ReplaceAll(text, "<", ":")
				text = strings.ReplaceAll(text, ">", "")
			}

			stackSlice = append(stackSlice, text)
			count++

			if !c.fullStackTrace && count > 3 {
				break
			}
		}

		ui.Log(ui.TraceLogger, "trace.instruction",
			"thread", c.threadID,
			"addr", c.programCounter,
			"opcode", strings.TrimSpace(instruction),
			"operand", operand,
			"stackPointer", c.stackPointer,
			"stack", stackSlice,
			"line", c.line,
			"module", c.GetModuleName())

		return
	}

	stack := c.formatStack(c.fullStackTrace)
	if !c.fullStackTrace && len(stack) > 80 {
		stack = stack[:80]
	}

	if len(instruction+operand) > 30 {
		ui.Log(ui.TraceLogger, "(%d) %18s %3d: %s",
			c.threadID, c.GetModuleName(), c.programCounter, instruction+" "+operand)
		ui.Log(ui.TraceLogger, "(%d) %18s %3s  %-30s stack[%2d]: %s",
			c.threadID, " ", " ", " ", c.stackPointer, stack)
	} else {
		ui.Log(ui.TraceLogger, "(%d) %18s %3d: %-30s stack[%2d]: %s",
			c.threadID, c.GetModuleName(), c.programCounter, instruction+" "+operand, c.stackPointer, stack)
	}
}
