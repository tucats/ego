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
	var tracing bool

	// We might be tracing because we're under control of the dashboard UI.
	if c.output != nil && c.tracing {
		tracing = true
	}

	// We might be doing this because the trace logger is enabled.
	if ui.IsActive(ui.TraceLogger) {
		tracing = true
	}

	if !tracing {
		return
	}

	instruction, operand := FormatInstruction(i)

	stackSlice := make([]string, 0)
	count := 0

	for i := c.stackPointer - 1; i >= 0; i-- {
		item := c.stack[i]
		text := data.Format(item)

		if _, isString := item.(string); !isString {
			text = strings.ReplaceAll(text, "<", ":{")
			text = strings.ReplaceAll(text, ">", "}")
		}

		stackSlice = append(stackSlice, text)
		count++

		if !c.fullStackTrace && count > 3 {
			break
		}
	}

	// if trace logger is inactive, we should send the output to the context io.Writer
	// So it can be gathered up by the dashboard.  Use the i18n package to format the
	// line for the output.
	if c.output != nil || !ui.IsActive(ui.TraceLogger) {
		text := ui.FormatLogMessage(ui.TraceLogger, "log.trace.instruction", ui.A{
			"thread":       c.threadID,
			"addr":         c.programCounter,
			"opcode":       strings.TrimSpace(instruction),
			"operand":      operand,
			"stackpointer": c.stackPointer,
			"stack":        stackSlice,
			"line":         c.line,
			"module":       c.GetModuleName()})

		text = ui.FormatJSONLogEntryAsText(text)

		// c.output is a Writer, so write the text to the output.
		c.output.Write([]byte(text + "\n"))
	} else {
		// Otherwise, use the standard log package to log the trace information.
		ui.Log(ui.TraceLogger, "trace.instruction", ui.A{
			"thread":       c.threadID,
			"addr":         c.programCounter,
			"opcode":       strings.TrimSpace(instruction),
			"operand":      operand,
			"stackpointer": c.stackPointer,
			"stack":        stackSlice,
			"line":         c.line,
			"module":       c.GetModuleName()})
	}
}
