package bytecode

import (
	"fmt"
	"reflect"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/symbols"
)

type optimizerOperation int

const (
	optNothing optimizerOperation = iota
	optStore
	optRead
	OptCount
	optRunConstantFragment
)

type placeholder struct {
	Name      string
	Operation optimizerOperation
	Register  int
	Value     interface{}
}

type optimization struct {
	Description string
	Debug       bool
	Pattern     []instruction
	Replacement []instruction
}

// optimize runs a peep-hold optimizer over the bytecode.
func (b *ByteCode) optimize(count int) (int, error) {
	startingSize := b.nextAddress

	// Figure out the maximum pattern size, since we'll need this for backing
	// up the bytecode scanner after each patch operation.
	maxPatternSize := 0
	for _, optimization := range optimizations {
		if max := len(optimization.Pattern); max > maxPatternSize {
			maxPatternSize = max
		}
	}

	// Starting at each sequential bytecode, see if any of the patterns
	// match.
	for idx := 0; idx < b.nextAddress; idx++ {
		found := false

		// Scan over all the available optimizations.
		for _, optimization := range optimizations {
			operandValues := map[string]placeholder{}
			registers := make([]interface{}, 5)
			found = true

			// Is there a branch INTO this pattern? If so, then it
			// cannot be optimized.
			for _, i := range b.instructions {
				if i.Operation > BranchInstructions {
					destination := data.Int(i.Operand)
					if destination >= idx && destination < idx+len(optimization.Pattern) {
						found = false

						break
					}
				}
			}

			if !found {
				continue
			}

			// Search each instruction in the pattern to see if it matches
			// with the instruction stream we are positioned at.
			for sourceIdx, sourceInstruction := range optimization.Pattern {
				if b.nextAddress <= idx+sourceIdx {
					found = false

					// This optimization can't fit because we're too near the end
					// so go on to the next optimization.
					break
				}

				i := b.instructions[idx+sourceIdx]

				if sourceInstruction.Operation != i.Operation {
					found = false

					// This optimization didn't match; go to next optimization
					break
				}

				// Debugging trap for optimization in "main"
				if sourceIdx == 0 && optimization.Debug && b.name == defs.Main {
					fmt.Printf("DEBUG breakpoint for %s, first operand = %v\n", optimization.Description, i.Operand)
				}

				// If the operands match between the instruction and the pattern,
				// we're good and should keep going...
				if reflect.DeepEqual(sourceInstruction.Operand, i.Operand) {
					continue
				}

				if token, ok := sourceInstruction.Operand.(placeholder); ok {
					value, inMap := operandValues[token.Name]
					if inMap {
						if value.Value == i.Operand {
							// no work to do
						} else if i.Operand != sourceInstruction.Operand {
							found = false

							continue
						}
					} else {
						switch token.Operation {
						case OptCount:
							increment := 1
							if i.Operand != nil {
								increment = data.Int(i.Operand)
							}
							registers[token.Register] = data.Int(registers[token.Register]) + increment

						case optStore:
							registers[token.Register] = i.Operand
						}

						operandValues[token.Name] = placeholder{Name: token.Name, Value: i.Operand}
					}
				}
			}

			// Does this optimization match?
			if found {
				if count == 0 && ui.IsActive(ui.OptimizerLogger) {
					ui.Log(ui.OptimizerLogger, "@@@ Optimizing bytecode %s @@@", b.name)
				}

				ui.Log(ui.OptimizerLogger, "Optimization found in %s: %s", b.name, optimization.Description)

				// Make a copy of the replacements, with the token values from the
				// source stream inserted as appropriate.
				replacements := []instruction{}

				for _, replacement := range optimization.Replacement {
					newInstruction := replacement

					if token, ok := replacement.Operand.(placeholder); ok {
						switch token.Operation {
						case optRunConstantFragment:
							v, err := b.executeFragment(idx, idx+len(optimization.Pattern))
							if err != nil {
								return 0, err
							}

							newInstruction.Operand = v

						case optRead:
							newInstruction.Operand = registers[token.Register]

						default:
							newInstruction.Operand = operandValues[token.Name].Value
						}
					}

					// Second slightly more complex case, where the replacement
					// consists of multiple tokens, any of which might be drawn
					// from the valuemap.
					if tokenArray, ok := replacement.Operand.([]interface{}); ok {
						newArray := []interface{}{}

						for _, item := range tokenArray {
							if token, ok := item.(placeholder); ok {
								newArray = append(newArray, operandValues[token.Name].Value)
							} else {
								newArray = append(newArray, item)
							}
						}

						newInstruction.Operand = newArray
					}

					replacements = append(replacements, newInstruction)
				}

				b.Patch(idx, len(optimization.Pattern), replacements)

				// Back up the pointer and continue, since we may now be part of
				// a previous pattern.
				idx = (idx - maxPatternSize) - 1

				if idx < 0 {
					idx = 0
				}

				count++
			}
		}
	}

	if count > 0 && ui.IsActive(ui.OptimizerLogger) && b.nextAddress != startingSize {
		ui.Log(ui.OptimizerLogger, "Found %d optimization(s) for net change in size of %d instructions", count, startingSize-b.nextAddress)
		ui.Log(ui.OptimizerLogger, "")
	}

	return count, nil
}

func (b *ByteCode) executeFragment(start, end int) (interface{}, error) {
	fragment := New("code fragment")

	for idx := start; idx < end; idx++ {
		i := b.instructions[idx]
		fragment.Emit(i.Operation, i.Operand)
	}

	fragment.Emit(Stop)

	s := symbols.NewSymbolTable("fragment")
	c := NewContext(s, fragment)
	c.typeStrictness = 0 // Assume strict typing

	if err := c.Run(); err != nil {
		return nil, err
	}

	return c.Pop()
}

func (b *ByteCode) Patch(start, deleteSize int, insert []instruction) {
	offset := deleteSize - len(insert)
	savedBytecodeLoggerState := ui.IsActive(ui.ByteCodeLogger)

	defer func() {
		ui.Active(ui.ByteCodeLogger, savedBytecodeLoggerState)
	}()

	if ui.IsActive(ui.OptimizerLogger) {
		ui.Active(ui.ByteCodeLogger, true)
		ui.Log(ui.OptimizerLogger, "Patching, existing code:")
		b.Disasm(start, start+deleteSize)
	}

	// Start by deleting the old instructions
	instructions := append(b.instructions[:start], insert...)
	instructions = append(instructions, b.instructions[start+deleteSize:]...)

	// Scan the instructions with destinations after the insertion and update jump offsets
	for i := 0; i < len(instructions); i++ {
		if instructions[i].Operation > BranchInstructions {
			destination := data.Int(instructions[i].Operand)
			if destination > start {
				instructions[i].Operand = destination - offset
			}
		}
	}

	b.instructions = instructions
	b.nextAddress = b.nextAddress - offset

	if ui.IsActive(ui.OptimizerLogger) {
		ui.Active(ui.ByteCodeLogger, true)
		ui.Log(ui.OptimizerLogger, "Patching, new code:")
		b.Disasm(start, start+len(insert))
	}
}
