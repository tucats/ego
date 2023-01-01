package bytecode

import (
	"fmt"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
)

type OptimizerOperation int

const (
	OptNothing OptimizerOperation = iota
	OptAdd
)

type OptimizerToken struct {
	Name      string
	Operation OptimizerOperation
	Value     interface{}
}

type Optimization struct {
	Description string
	Debug       bool
	Source      []Instruction
	Replacement []Instruction
}

// Optimize runs a peep-hold optimizer over the bytecode.
func (b *ByteCode) Optimize() (int, *errors.EgoError) {
	startingSize := b.emitPos
	count := 0

	// Figure out the maximum pattern size, since we'll need this for backing
	// up the bytecode scanner after each patch operation.
	maxPatternSize := 0
	for _, optimization := range Optimizations {
		if max := len(optimization.Source); max > maxPatternSize {
			maxPatternSize = max
		}
	}

	// Starting at each sequential bytecode, see if any of the patterns
	// match.
	for idx := 0; idx < b.emitPos; idx++ {
		found := false

		// Scan over all the available optimizations.
		for _, optimization := range Optimizations {
			operandValues := map[string]OptimizerToken{}
			integerAccumulator := 0
			found = true

			if optimization.Debug && b.Name == "main" {
				fmt.Println("DEBUG breakpoint for " + optimization.Description)
			}

			// Search each instruction in the pattern to see if it matches
			// with the instruction stream we are positioned at.
			for sourceIdx, sourceInstruction := range optimization.Source {
				if b.emitPos <= idx+sourceIdx {
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

				if token, ok := sourceInstruction.Operand.(OptimizerToken); ok {
					value, inMap := operandValues[token.Name]
					if inMap {
						if value.Value == i.Operand {
							// no work to do
						} else if i.Operand != sourceInstruction.Operand {
							found = false

							continue
						}
					} else {
						if token.Operation == OptAdd {
							if _, ok := i.Operand.(int); ok {
								integerAccumulator += datatypes.GetInt(i.Operand)
							}
						}

						operandValues[token.Name] = OptimizerToken{Name: token.Name, Value: i.Operand}
					}
				}
			}

			// Does this optimization match?
			if found {
				if count == 0 && ui.LoggerIsActive(ui.OptimizerLogger) {
					ui.Debug(ui.OptimizerLogger, "@@@ Optimizing bytecode %s @@@", b.Name)
					ui.Debug(ui.OptimizerLogger, "    Code before optimizations:")

					oldBytecodeLoggingStatus := ui.LoggerIsActive(ui.ByteCodeLogger)

					ui.SetLogger(ui.ByteCodeLogger, true)
					b.Disasm()
					ui.SetLogger(ui.ByteCodeLogger, oldBytecodeLoggingStatus)
					ui.Debug(ui.OptimizerLogger, "")
				}

				ui.Debug(ui.OptimizerLogger, "Optimization found in %s: %s", b.Name, optimization.Description)

				// Make a copy of the replacements, with the token values from the
				// source stream inserted as appropriate.
				replacements := []Instruction{}

				for _, replacement := range optimization.Replacement {
					newInstruction := replacement

					if token, ok := replacement.Operand.(OptimizerToken); ok {
						switch token.Operation {
						case OptAdd:
							newInstruction.Operand = integerAccumulator

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
							if token, ok := item.(OptimizerToken); ok {
								newArray = append(newArray, operandValues[token.Name].Value)
							} else {
								newArray = append(newArray, item)
							}
						}

						newInstruction.Operand = newArray
					}

					replacements = append(replacements, newInstruction)
				}

				b.Patch(idx, len(optimization.Source), replacements)

				// Back up the pointer and continue, since we may now be part of
				// a previous pattern.
				idx = idx - maxPatternSize
				if idx < 0 {
					idx = 0
				}

				count++
			}

			// Clear out the operand values after each check.
			integerAccumulator = 0
		}
	}

	if count > 0 && ui.LoggerIsActive(ui.OptimizerLogger) {
		ui.Debug(ui.OptimizerLogger, "Found %d optimization(s) for net change in size of %d instructions", count, startingSize-b.emitPos)
		oldBytecodeLoggingStatus := ui.LoggerIsActive(ui.ByteCodeLogger)

		ui.Debug(ui.OptimizerLogger, "    Code after  optimizations:")

		ui.SetLogger(ui.ByteCodeLogger, true)
		b.Disasm()
		ui.SetLogger(ui.ByteCodeLogger, oldBytecodeLoggingStatus)
		ui.Debug(ui.OptimizerLogger, "")
	}

	return count, nil
}

func (b *ByteCode) Patch(start, deleteSize int, insert []Instruction) {
	offset := deleteSize - len(insert)

	// Start by deleting the old instructions
	instructions := append(b.instructions[:start], insert...)
	instructions = append(instructions, b.instructions[start+deleteSize:]...)

	// Scan the instructions with destinations after the insertion and update jump offsets
	for i := 0; i < len(instructions); i++ {
		if instructions[i].Operation > BranchInstructions {
			destination := datatypes.GetInt(instructions[i].Operand)
			if destination > start {
				instructions[i].Operand = destination - offset
			}
		}
	}

	b.instructions = instructions
	b.emitPos = b.emitPos - offset
}
