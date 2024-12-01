package bytecode

import (
	"reflect"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
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

type empty struct{}

type placeholder struct {
	Name         string
	MustBeString bool
	Operation    optimizerOperation
	Register     int
	Value        interface{}
}

type optimization struct {
	Description string
	Debug       bool
	Disable     bool
	Pattern     []instruction
	Replacement []instruction
}

// optimize runs a peep-hole optimizer over the bytecode.
func (b *ByteCode) optimize(count int) (int, error) {
	if b.optimized {
		return 0, nil
	}

	// Remember how bit this was when we began, for reporting purposes.
	startingSize := b.nextAddress

	// Figure out the maximum pattern size, since we'll need this for backing
	// up the bytecode scanner after each patch operation.
	maxPatternSize := 0
	for _, optimization := range optimizations {
		if optPatternSize := len(optimization.Pattern); optPatternSize > maxPatternSize {
			maxPatternSize = optPatternSize
		}
	}

	// Starting at each sequential bytecode, see if any of the patterns
	// match.
	for idx := 0; idx < b.nextAddress; idx++ {
		found := false

		// Scan over all the available optimizations that are active.
		for _, optimization := range optimizations {
			if optimization.Disable {
				continue
			}

			operandValues := map[string]placeholder{}
			registers := make([]interface{}, 5)
			found = true

			// Is there a branch INTO this pattern? If so, then it
			// cannot be optimized.
			if branchTargetInPatter(b, idx, optimization, found) {
				continue
			}

			// Search each instruction in the pattern to see if it matches
			// with the instruction stream we are positioned at.
			found = doesPatternMatch(optimization, b, idx, found, operandValues, registers)

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
					} else {
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

	b.optimized = true

	return count, nil
}

// Determine if the current operation is the start of this optimization pattern. Compares
// the operation and operands, and ensures there is enough room to fit the pattern.
func doesPatternMatch(optimization optimization, b *ByteCode, idx int, found bool, operandValues map[string]placeholder, registers []interface{}) bool {
	for sourceIdx, sourceInstruction := range optimization.Pattern {
		// This optimization can't fit because we're too near the end
		// so go on to the next optimization.
		if b.nextAddress <= idx+sourceIdx {
			found = false

			break
		}

		i := b.instructions[idx+sourceIdx]

		// If the source instruction requires an empty operand and the operand isn't nil,
		// then we skip this optimization.
		if _, isEmpty := sourceInstruction.Operand.(empty); isEmpty && i.Operand != nil {
			found = false

			break
		}

		// This optimization didn't match; go to next optimization
		if sourceInstruction.Operation != i.Operation {
			found = false

			break
		}

		// This optimization didn't match; go to next optimization
		if pattern, ok := sourceInstruction.Operand.(placeholder); ok {
			if pattern.MustBeString {
				if _, ok := i.Operand.(string); !ok {
					found = false

					break
				}
			}
		}

		if reflect.DeepEqual(sourceInstruction.Operand, i.Operand) {
			continue
		}

		if token, ok := sourceInstruction.Operand.(placeholder); ok {
			value, inMap := operandValues[token.Name]
			if inMap {
				if value.Value == i.Operand {

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

	return found
}

func branchTargetInPatter(b *ByteCode, idx int, optimization optimization, found bool) bool {
	for _, i := range b.instructions {
		if i.Operation > BranchInstructions {
			destination := data.Int(i.Operand)
			if destination >= idx && destination < idx+len(optimization.Pattern) {
				found = false

				break
			}
		}
	}

	return found
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
	c.typeStrictness = defs.StrictTypeEnforcement // Assume strict typing

	if err := c.Run(); err != nil && !errors.Equal(err, errors.ErrStop) {
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
