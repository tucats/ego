package bytecode

import (
	"log"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
	"github.com/tucats/ego/tokenizer"
)

// growthIncrement indicates the number of elements to add to the
// opcode array when storage is exhausted in the current array.
const growthIncrement = 32

// initialOpcodeSize is the initial size of the emit buffer.
const initialOpcodeSize = 20

// initialStackSize is the initial stack size.
const initialStackSize = 16

// firstOptimizerLogMessage is a flag that indicates if this is the first time the
// optimizer is being invoked, but has been turned off by configuration, and
// the optimizer log is active. In this case, we put out (once) a message saying
// logging is suppressed by configuration option.
var firstOptimizerLogMessage = true

// ByteCode contains the context of the execution of a bytecode stream. Note that
// there is a dependency in format.go on the name of the "Declaration" variable.
// PLEASE NOTE that Name must be exported because reflection is used to format
// opaque pointers to bytecodes in the low-level formatter.
type ByteCode struct {
	// The name of the bytecode stream. This is usually the name of the function
	// or source file that generated this bytecode.
	name string

	// The array of bytecode instructions to execute.
	instructions []instruction

	// The next address in the instructions array in which to write the next
	// instruction. This allows the array to be extended in chunks dynamically.
	nextAddress int

	// If this is a bytecode stream for a function, this contains the function's
	// declaration object (indicating expected parameters, types, and return values).
	declaration *data.Declaration

	// List of targets written by this lvalue, when this is an lvalue
	targets []string

	// When the bytecode is an LValue that will be used to store a value, this contains
	// the number of store operations in the bytecode. This is used by the assignment
	// compiler to determine if the LValue is handling tuples versus a single value.
	storeCount int

	// This is true if the bytecode array has been truncated to the actual length of
	// the bytecode stream (for memory efficiency).
	sealed bool

	// True if the peep=hole optimizer has been run on this bytecode object. Adding or
	// modifying the bytecode turns this flag off.
	optimized bool

	// If this is the bytecode for a function literal, this is set to true.
	literal bool
}

// String formats a bytecode as a function declaration string.
func (b *ByteCode) String() string {
	if b.declaration != nil {
		return b.declaration.String()
	}

	return b.name + "()"
}

// Set the flag indicating if this bytecode is for a function literal.
func (b *ByteCode) Literal(flag bool) *ByteCode {
	b.literal = flag

	return b
}

// Get the flag indicating if this bytecode is for a function literal.
func (b *ByteCode) IsLiteral() bool {
	return b.literal
}

// Size returns the number of instructions in the bytecode object. This may
// not be the same as the size of the bytecode instruction array.
func (b *ByteCode) Size() int {
	return b.nextAddress
}

// Return the declaration object from the bytecode. This is primarily
// used in routines that format information about the bytecode. If you
// change the name of this function, you will also need to update the
// MethodByName() calls for this same function name.
func (b *ByteCode) Declaration() *data.Declaration {
	return b.declaration
}

// Set the declaration object for this bytecode.
func (b *ByteCode) SetDeclaration(fd *data.Declaration) *ByteCode {
	b.declaration = fd

	return b
}

// Get the number of store operations in this bytecode stream.
func (b *ByteCode) StoreCount() int {
	return b.storeCount
}

// Get the name of this bytecode stream.
func (b *ByteCode) Name() string {
	if b.name == "" {
		return defs.Anon
	}

	return b.name
}

// Set the name of the bytecode stream.
func (b *ByteCode) SetName(name string) *ByteCode {
	b.name = name

	return b
}

// New generates and initializes a new bytecode stream.
func New(name string) *ByteCode {
	if name == "" {
		name = defs.Anon
	}

	return &ByteCode{
		name:         name,
		instructions: make([]instruction, initialOpcodeSize),
		nextAddress:  0,
		sealed:       false,
		optimized:    false,
	}
}

// EmitAt emits a single instruction. The opcode is required, and can optionally
// be followed by an instruction operand (based on whichever instruction)
// is issued. This stores the instruction at the given location in the bytecode
// array, but does not affect the emit position unless this operation required
// expanding the bytecode storage.
func (b *ByteCode) EmitAt(address int, opcode Opcode, operands ...interface{}) {
	// If the output capacity is too small, expand it.
	for address >= len(b.instructions) {
		b.instructions = append(b.instructions, make([]instruction, growthIncrement)...)
	}

	if address > b.nextAddress {
		b.nextAddress = address
	}

	// If this is a Store operation, count it. This is used to handle
	// assignments to tuples.
	if opcode == Store || opcode == CreateAndStore {
		b.storeCount++
	}

	instruction := instruction{Operation: opcode}

	// If there is one operand, store that in the instruction. If
	// there are multiple operands, make them into an array.
	if len(operands) > 0 {
		if len(operands) > 1 {
			instruction.Operand = operands
		} else {
			instruction.Operand = operands[0]
		}
	}

	// If the operand is a token, use the spelling of the token
	// as the value. If it's an integer or floating point value,
	// convert the token to a value.
	if token, ok := instruction.Operand.(tokenizer.Token); ok {
		var err error

		text := token.Spelling()
		if token.IsClass(tokenizer.IntegerTokenClass) {
			if instruction.Operand, err = data.Int(text); err != nil {
				log.Fatalf("invalid integer token: %q", text)
			}
		} else if token.IsClass(tokenizer.FloatTokenClass) {
			if instruction.Operand, err = data.Float64(text); err != nil {
				log.Fatalf("invalid float token: %q", text)
			}
		} else {
			instruction.Operand = text
		}
	}

	b.instructions[address] = instruction
	b.sealed = false
	b.optimized = false
}

// Emit emits a single instruction. The opcode is required, and can optionally
// be followed by an instruction operand (based on whichever instruction)
// is issued. The instruction is emitted at the current "next address" of
// the bytecode object, which is then incremented.
func (b *ByteCode) Emit(opcode Opcode, operands ...interface{}) {
	b.EmitAt(b.nextAddress, opcode, operands...)

	b.nextAddress++
}

// Delete bytecodes from the bytecode array by address. If the address is
// invalid, this is a no-op.
func (b *ByteCode) Delete(position int) {
	// If it's just the last item, we can just reset the next address.
	if position == b.nextAddress-1 {
		b.nextAddress = b.nextAddress - 1
	} else if position >= 0 && position < len(b.instructions) {
		b.instructions = append(b.instructions[:position], b.instructions[position+1:]...)
		if b.nextAddress > len(b.instructions) {
			b.nextAddress = len(b.instructions)
		}

		// Since we deleted something, any branch to a bytecode after this
		// position must be adjusted by one.
		for i := 0; i < b.nextAddress; i++ {
			if b.instructions[i].Operation >= BranchInstructions {
				if addr, ok := b.instructions[i].Operand.(int); ok {
					b.instructions[i].Operand = addr - 1
				}
			}
		}

		b.optimized = false
	}
}

// Truncate the output array to the current bytecode size. This is also
// where we will optionally run an optimizer.
func (b *ByteCode) Seal() *ByteCode {
	if b == nil {
		return nil
	}

	// If this bytecode block is already sealed, we have no work to do.
	if b.sealed {
		return b
	}

	b.sealed = true

	b.instructions = b.instructions[:b.nextAddress]

	useOptimizer := settings.GetBool(defs.OptimizerSetting)

	if ui.IsActive(ui.OptimizerLogger) && firstOptimizerLogMessage && !useOptimizer {
		firstOptimizerLogMessage = false

		ui.Log(ui.OptimizerLogger, "optimizer.disabled", nil)
	}

	// Optionally run optimizer.
	if useOptimizer {
		_, _ = b.optimize(0)
	}

	return b
}

// Mark returns the address of the next instruction to be emitted. Use
// this BEFORE a call to Emit() if using it for branch address fixups
// later.
func (b *ByteCode) Mark() int {
	return b.nextAddress
}

// Truncate the bytecode to the given mark location. This is used to prune
// away bytecode that was generated before an error was found that can be
// backed out.
func (b *ByteCode) Truncate(mark int) *ByteCode {
	b.nextAddress = mark

	b.optimized = false
	b.sealed = false

	return b
}

// SetAddressHere sets the current address as the destination of the
// instruction at the marked location. This is used for address
// fixups, typically for forward branches.
func (b *ByteCode) SetAddressHere(mark int) error {
	return b.SetAddress(mark, b.nextAddress)
}

// SetAddress sets the given value as the target of the marked
// instruction. This is often used when an address has been
// saved and we need to update a branch destination, usually
// for a backwards branch operation.
func (b *ByteCode) SetAddress(mark int, address int) error {
	if mark > b.nextAddress || mark < 0 {
		return errors.ErrInvalidBytecodeAddress
	}

	instruction := b.instructions[mark]
	instruction.Operand = address
	b.instructions[mark] = instruction

	return nil
}

// Append appends another bytecode set to the current bytecode,
// and updates all the branch references within that code to
// reflect the new base location for the code segment.
func (b *ByteCode) Append(a *ByteCode) {
	if a == nil {
		return
	}

	offset := b.nextAddress

	for _, instruction := range a.instructions[:a.nextAddress] {
		if instruction.Operation > BranchInstructions {
			instruction.Operand = data.IntOrZero(instruction.Operand) + offset
		}

		b.Emit(instruction.Operation, instruction.Operand)
	}

	b.sealed = false
	b.optimized = false
}

// Instruction retrieves the instruction at the given address.
func (b *ByteCode) Instruction(address int) *instruction {
	if address < 0 || address >= len(b.instructions) {
		return nil
	}

	return &(b.instructions[address])
}

// Run generates a one-time context for executing this bytecode,
// and then executes the code.
func (b *ByteCode) Run(s *symbols.SymbolTable) error {
	c := NewContext(s, b)

	return c.Run()
}

// Call generates a one-time context for executing this bytecode,
// and returns a value as well as an error condition if there was
// one from executing the code.
func (b *ByteCode) Call(s *symbols.SymbolTable) (interface{}, error) {
	c := NewContext(s, b)

	if err := c.Run(); err != nil {
		return nil, err
	}

	return c.Pop()
}

// Opcodes returns the opcode list for this bytecode array.
func (b *ByteCode) Opcodes() []instruction {
	return b.instructions[:b.nextAddress]
}

// Remove removes an instruction from the bytecode. The address is
// >= 0 it is the absolute address of the instruction to remove.
// Otherwise, it is the offset from the end of the bytecode to remove.
func (b *ByteCode) Remove(address int) {
	if address >= 0 {
		b.instructions = append(b.instructions[:address], b.instructions[address+1:]...)
	} else {
		offset := b.nextAddress - address
		b.instructions = append(b.instructions[:offset], b.instructions[offset+1:]...)
	}

	b.nextAddress = b.nextAddress - 1
	b.optimized = false
	b.sealed = false
}

// ClearLineNumbers scans the bytecode and removes the AtLine numbers
// in the code so far. This is done when the @line directive resets the
// line number; all previous line numbers are no longer valid and are
// set to zero.
func (b *ByteCode) ClearLineNumbers() {
	for index, instruction := range b.instructions {
		if index == len(b.instructions)-1 {
			break
		}

		if instruction.Operation == AtLine {
			instruction.Operand = 0
			b.instructions[index] = instruction
		}
	}
}
