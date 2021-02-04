package bytecode

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/util"
)

// Disasm prints out a representation of the bytecode for debugging purposes.
func (b *ByteCode) Disasm() {
	ui.Debug(ui.ByteCodeLogger, "*** Disassembly %s", b.Name)

	for n := 0; n < b.emitPos; n++ {
		i := b.instructions[n]
		op := FormatInstruction(i)
		ui.Debug(ui.ByteCodeLogger, "%4d: %s", n, op)
	}

	ui.Debug(ui.ByteCodeLogger, "*** Disassembled %d instructions", b.emitPos)
}

// FormatInstruction formats a single instruction as a string.
func FormatInstruction(i Instruction) string {
	opname, found := instructionNames[i.Operation]

	// What is the maximum opcode name length?
	width := 0

	for _, k := range instructionNames {
		if len(k) > width {
			width = len(k)
		}
	}

	if !found {
		opname = fmt.Sprintf("Unknown %d", i.Operation)
	}

	opname = (opname + strings.Repeat(" ", width))[:width]
	f := util.Format(i.Operand)

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

		opname, found := instructionNames[i.Operation]
		if !found {
			opname = fmt.Sprintf("Unknown %d", i.Operation)
		}

		f := util.Format(i.Operand)
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
