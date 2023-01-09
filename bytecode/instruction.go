package bytecode

import (
	"fmt"

	"github.com/tucats/ego/data"
)

// instruction contains the information about a single bytecode.
type instruction struct {
	Operation Opcode
	Operand   interface{}
}

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
