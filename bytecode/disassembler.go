package bytecode

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
)

// Disasm prints out a representation of the bytecode for debugging purposes.
func (b *ByteCode) Disasm(ranges ...int) {
	usingRange := false
	start := 0

	if len(ranges) > 0 {
		start = ranges[0]
		usingRange = true
	}

	end := b.emitPos
	if len(ranges) > 1 {
		end = ranges[1]
	}

	if ui.IsActive(ui.ByteCodeLogger) {
		if !usingRange {
			ui.Debug(ui.ByteCodeLogger, "*** Disassembly %s", b.Name)
		}

		scopePad := 0

		for n := start; n < end; n++ {
			i := b.instructions[n]
			if i.Operation == PopScope && scopePad > 0 {
				scopePad = scopePad - 1
			}

			op := FormatInstruction(i)
			ui.Debug(ui.ByteCodeLogger, "%4d: %s%s", n, strings.Repeat("| ", scopePad), op)

			if i.Operation == PushScope {
				scopePad = scopePad + 1
			}
		}

		if !usingRange {
			ui.Debug(ui.ByteCodeLogger, "*** Disassembled %d instructions", end-start)
		}
	}
}

var maxInstructionNameWidth = 0

// FormatInstruction formats a single instruction as a string.
func FormatInstruction(i Instruction) string {
	opname, found := opcodeNames[i.Operation]

	// What is the maximum opcode name length?
	if maxInstructionNameWidth == 0 {
		for _, k := range opcodeNames {
			if len(k) > maxInstructionNameWidth {
				maxInstructionNameWidth = len(k)
			}
		}
	}

	if !found {
		opname = fmt.Sprintf("Unknown %d", i.Operation)
	}

	opname = (opname + strings.Repeat(" ", maxInstructionNameWidth))[:maxInstructionNameWidth]
	f := datatypes.Format(i.Operand)
	f = strings.ReplaceAll(f, "\n", "\\n")
	f = strings.ReplaceAll(f, "\t", "\\t")

	if i.Operand == nil {
		f = ""
	}

	if i.Operation >= BranchInstructions {
		f = "@" + f
	}

	return opname + " " + f
}

// Format formats an array of bytecodes.
func Format(opcodes []Instruction) string {
	var b strings.Builder

	b.WriteRune('[')

	for n, i := range opcodes {
		if n > 0 {
			b.WriteRune(',')
		}

		opname, found := opcodeNames[i.Operation]
		if !found {
			opname = fmt.Sprintf("Unknown %d", i.Operation)
		}

		f := datatypes.Format(i.Operand)
		if i.Operand == nil {
			f = ""
		}

		if i.Operation >= BranchInstructions {
			f = "@" + f
		}

		b.WriteString(opname)

		if len(f) > 0 {
			b.WriteRune(' ')
			b.WriteString(f)
		}
	}

	b.WriteRune(']')

	return b.String()
}
