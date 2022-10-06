package bytecode

import (
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// GrowOpcodesBy indicates the number of elements to add to the
// opcode array when storage is exhausted in the current array.
const GrowOpcodesBy = 50

// InitialOpcodeSize is the initial size of the emit buffer.
const InitialOpcodeSize = 20

// InitialStackSize is the initial stack size.
const InitialStackSize = 50

// BranchInstruction is the minimum value for a branch instruction, which has
// special meaning during relocation and linking.
const BranchInstruction = 2000

// BuiltinInstructions defines the lowest number (other than Stop) of the
// builtin instructions provided by the bytecode package. User instructions
// are added between 1 and this value.
const BuiltinInstructions = BranchInstruction - 1000

// ErrorVariableName is the name of the local variable created for a
// catch-block of a try/catch construct. The variable contains an error.
const ErrorVariableName = "_error"

// Instruction contains the information about a single bytecode.
type Instruction struct {
	Operation OpcodeID
	Operand   interface{}
}

// ByteCode contains the context of the execution of a bytecode stream.
type ByteCode struct {
	Name         string
	instructions []Instruction
	emitPos      int
	Declaration  *datatypes.FunctionDeclaration
}

// New generates and initializes a new bytecode.
func New(name string) *ByteCode {
	bc := ByteCode{
		Name:         name,
		instructions: make([]Instruction, InitialOpcodeSize),
		emitPos:      0,
	}

	return &bc
}

// Emit emits a single instruction. The opcode is required, and can optionally
// be followed by an instruction operand (based on whichever instruction)
// is issued.
func (b *ByteCode) Emit(opcode OpcodeID, operands ...interface{}) {
	// If the output capacity is too small, expand it.
	if b.emitPos >= len(b.instructions) {
		b.instructions = append(b.instructions, make([]Instruction, GrowOpcodesBy)...)
	}

	i := Instruction{Operation: opcode}

	// If there is one operand, store that in the instruction. If
	// there are multiple operands, make them into an array.
	if len(operands) > 0 {
		if len(operands) > 1 {
			i.Operand = operands
		} else {
			i.Operand = operands[0]
		}
	}

	b.instructions[b.emitPos] = i
	b.emitPos = b.emitPos + 1
}

// Truncate the output array to the current bytecode size.
func (b *ByteCode) Seal() {
	b.instructions = b.instructions[:b.emitPos]
}

// Mark returns the address of the instruction about to be emitted.
func (b *ByteCode) Mark() int {
	return b.emitPos
}

// SetAddressHere sets the current address as the target of the marked
// instruction.
func (b *ByteCode) SetAddressHere(mark int) *errors.EgoError {
	return b.SetAddress(mark, b.emitPos)
}

// SetAddress sets the given value as the target of the marked
// instruction.
func (b *ByteCode) SetAddress(mark int, address int) *errors.EgoError {
	if mark > b.emitPos || mark < 0 {
		return b.NewError(errors.ErrInvalidBytecodeAddress)
	}

	i := b.instructions[mark]
	i.Operand = address
	b.instructions[mark] = i

	return nil
}

// Append appends another bytecode set to the current bytecode,
// and updates all the link references.
func (b *ByteCode) Append(a *ByteCode) {
	if a == nil {
		return
	}

	base := b.emitPos

	for _, i := range a.instructions[:a.emitPos] {
		if i.Operation > BranchInstructions {
			i.Operand = datatypes.GetInt(i.Operand) + base
		}

		b.Emit(i.Operation, i.Operand)
	}
}

func (b *ByteCode) GetInstruction(pos int) *Instruction {
	if pos < 0 || pos >= len(b.instructions) {
		return nil
	}

	return &(b.instructions[pos])
}

// DefineInstruction adds a user-defined instruction to the bytecode
// set.
func DefineInstruction(opcode OpcodeID, name string, implementation OpcodeHandler) *errors.EgoError {
	// First, make sure this isn't a duplicate
	if _, found := dispatch[opcode]; found {
		return errors.New(errors.ErrOpcodeAlreadyDefined).Context(opcode)
	}

	instructionNames[opcode] = name
	dispatch[opcode] = implementation

	return nil
}

// Run generates a one-time context for executing this bytecode.
func (b *ByteCode) Run(s *symbols.SymbolTable) *errors.EgoError {
	c := NewContext(s, b)

	return c.Run()
}

// Call generates a one-time context for executing this bytecode,
// and returns a value as well as an error.
func (b *ByteCode) Call(s *symbols.SymbolTable) (interface{}, *errors.EgoError) {
	c := NewContext(s, b)

	err := c.Run()
	if !errors.Nil(err) {
		return nil, err
	}

	return c.Pop()
}

// Opcodes returns the opcode list for this bytecode array.
func (b *ByteCode) Opcodes() []Instruction {
	return b.instructions[:b.emitPos]
}

// Remove removes an instruction from the bytecode. The position is either
// >= 0 in which case it is absent, else if it is < 0 it is the offset
// from the end of the bytecode.
func (b *ByteCode) Remove(n int) {
	if n >= 0 {
		b.instructions = append(b.instructions[:n], b.instructions[n+1:]...)
	} else {
		n = b.emitPos - n
		b.instructions = append(b.instructions[:n], b.instructions[n+1:]...)
	}

	b.emitPos = b.emitPos - 1
}

// NewError creates a new ByteCodeErr using the message string and any
// optional arguments that are formatted using the message string.
func (b *ByteCode) NewError(err error, args ...interface{}) *errors.EgoError {
	r := errors.New(err)

	if len(args) > 0 {
		_ = r.Context(args[0])
	}

	return r
}

func (b *ByteCode) ClearLineNumbers() {
	for n, i := range b.instructions {
		if n == len(b.instructions)-1 {
			break
		}

		if i.Operation == AtLine {
			i.Operand = 0
			b.instructions[n] = i
		}
	}
}
