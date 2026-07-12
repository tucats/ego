package bytecode

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
)

// maxInstructionNameWidth is the maximum width of an instruction name. If set
// to zero, there is no limit on the length.
var maxInstructionNameWidth = 0

// Disasm prints out a representation of the bytecode for debugging purposes.
//
// By default (force=false) this only produces output when the bytecode
// logger (ui.ByteCodeLogger) is active process-wide, since formatting and
// printing every instruction is expensive and this is called frequently
// during normal compilation. Passing force=true bypasses that check and
// always prints the disassembly, regardless of whether the logger is
// active -- this is for callers such as the @compile directive's
// "bytecode"/"disasm" option, which want output for one specific call site
// without affecting anything else.
//
// force must NOT be implemented by temporarily toggling the logger's
// active flag (e.g. ui.Active(ui.ByteCodeLogger, true) then restoring it)
// around the call. That flag is process-global and shared by every
// goroutine; flipping it here would race with, and could unexpectedly
// enable or suppress bytecode logging for, any other goroutine compiling
// or disassembling code concurrently. Instead, force selects ui.WriteLog
// (which always emits) in place of ui.Log (which emits only when the class
// is active), so the global flag itself is never touched.
func (b *ByteCode) Disasm(force bool, ranges ...int) {
	var (
		usingRange bool
		start      int
	)

	if b == nil {
		return
	}

	// If a starting address is specified, use it.
	if len(ranges) > 0 {
		start = ranges[0]
		usingRange = true
	}

	// If an ending address is specified, use it.
	end := b.nextAddress
	if len(ranges) > 1 {
		end = ranges[1]
	}

	// Skip the (possibly expensive) formatting and iteration work entirely
	// if we're not forced and the bytecode logger is not active. This
	// preserves the original fast-path behavior for the normal, non-forced
	// case, which is called frequently during compilation.
	if !force && !ui.IsActive(ui.ByteCodeLogger) {
		return
	}

	// log is the write function to use for every line below: WriteLog
	// always emits regardless of the logger's active state (needed when
	// force is true, since the logger may genuinely not be active), while
	// Log emits only when the class is active (the original, non-forced
	// behavior).
	log := ui.Log
	if force {
		log = ui.WriteLog
	}

	if !usingRange {
		log(ui.ByteCodeLogger, "bytecode.disasm", ui.A{
			"name": b.name})
	}

	scopePad := 0

	// Iterate over the instructions, printing them out.
	for n := start; n < end; n++ {
		i := b.instructions[n]
		if i.Operation == PopScope && scopePad > 0 {
			scopePad = scopePad - 1
		}

		op, operand := FormatInstruction(i)

		log(ui.ByteCodeLogger, "bytecode.instruction", ui.A{
			"addr":    n,
			"depth":   scopePad,
			"op":      strings.TrimSpace(op),
			"operand": operand})

		if i.Operation == PushScope {
			scopePad = scopePad + 1
		}
	}

	// If we were not given a range, add a summary line indicating how many
	// instructions were disassembled.
	if !usingRange {
		log(ui.ByteCodeLogger, "bytecode.count", ui.A{
			"count": end - start})
	}
}

// FormatInstruction formats a single instruction as a string.
func FormatInstruction(i instruction) (string, string) {
	opName, found := opcodeNames[i.Operation]

	// What is the maximum opcode name length?
	if maxInstructionNameWidth == 0 {
		for _, k := range opcodeNames {
			if len(k) > maxInstructionNameWidth {
				maxInstructionNameWidth = len(k)
			}
		}
	}

	if !found {
		opName = fmt.Sprintf("Unknown %d", i.Operation)
	}

	// Format the operand. If it contains newlines or tabs, escape them.
	opName = (opName + strings.Repeat(" ", maxInstructionNameWidth))[:maxInstructionNameWidth]
	f := data.Format(i.Operand)
	f = strings.ReplaceAll(f, "\n", "\\n")
	f = strings.ReplaceAll(f, "\t", "\\t")

	if i.Operand == nil {
		f = ""
	}

	// If this is a branch instruction, add the @ prefix to the operand so
	// it is recognized as an address.
	if i.Operation >= BranchInstructions {
		f = "@" + f
	}

	return opName, f
}

// Format formats an array of bytecodes.
func Format(opcodes []instruction) string {
	var b strings.Builder

	b.WriteRune('[')

	for n, i := range opcodes {
		if n > 0 {
			b.WriteRune(',')
		}

		opName, found := opcodeNames[i.Operation]
		if !found {
			opName = fmt.Sprintf("Unknown %d", i.Operation)
		}

		f := data.Format(i.Operand)
		if i.Operand == nil {
			f = ""
		}

		if i.Operation >= BranchInstructions {
			f = "@" + f
		}

		b.WriteString(opName)

		if len(f) > 0 {
			b.WriteRune(' ')
			b.WriteString(f)
		}
	}

	b.WriteRune(']')

	return b.String()
}
