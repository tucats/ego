package bytecode

import (
	"fmt"

	"github.com/tucats/ego/datatypes"
)

// Instruction contains the information about a single bytecode.
type Instruction struct {
	Operation OpcodeID
	Operand   interface{}
}

func (i Instruction) String() string {
	name, found := instructionNames[i.Operation]
	if !found {
		name = fmt.Sprintf("Unknown(%d)", i.Operation)
	}

	operand := ""
	if i.Operand != nil {
		operand = " " + datatypes.Format(i.Operand)
	}

	return name + operand
}
