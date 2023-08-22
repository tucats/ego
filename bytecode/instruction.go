package bytecode

import (
	"fmt"

	"github.com/tucats/ego/data"
)

// instruction contains the information about a single bytecode.
type instruction struct {
	// The opcode for this instruction. This is an integer value.
	Operation Opcode

	// The operand for this instruction. This is an interface value, and
	// can contain a variety of types depending on the opcode.
	Operand interface{}
}

// String returns a string representation of an instruction.
func (i instruction) String() string {
	name, found := opcodeNames[i.Operation]
	if !found {
		name = fmt.Sprintf("Unknown(%d)", i.Operation)
	}

	if i.Operand != nil {
		return name + " " + data.Format(i.Operand)
	}

	return name
}
