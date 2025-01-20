package bytecode

import "github.com/tucats/ego/app-cli/ui"

// Trace instruction traces a single instruction in the current context. It formats a TRACE
// log line that contains the instruction, arguments, and current stack information. If the
// trace logger is not active, no work is done.
func traceInstruction(c *Context, i instruction) {
	if !ui.IsActive(ui.TraceLogger) {
		return
	}

	instruction, operand := FormatInstruction(i)

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
