package bytecode

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/datatypes"
	"github.com/tucats/ego/errors"
)

type OptimizerToken struct {
	Name  string
	Value interface{}
}

type Optimization struct {
	Description string
	Source      []Instruction
	Replacement []Instruction
}

var Optimizations = []Optimization{
	{
		Description: "Create and store",
		Source: []Instruction{
			{
				Operation: SymbolCreate,
				Operand:   OptimizerToken{Name: "symbolName"},
			},
			{
				Operation: Store,
				Operand:   OptimizerToken{Name: "symbolName"},
			},
		},
		Replacement: []Instruction{
			{
				Operation: CreateAndStore,
				Operand:   OptimizerToken{Name: "symbolName"},
			},
		},
	},
}

// Optimize runs a peep-hold optimizer over the bytecode.
func (b *ByteCode) Optimize() *errors.EgoError {
	startingSize := b.emitPos
	count := 0

	for _, optimization := range Optimizations {
		tokens := map[string]OptimizerToken{}

		for idx := 0; idx < len(b.instructions); idx++ {
			found := true

			for sourceIdx, sourceInstruction := range optimization.Source {
				if b.emitPos < idx+sourceIdx {
					found = false

					break
				}

				i := b.instructions[idx+sourceIdx]

				if sourceInstruction.Operation != i.Operation {
					found = false

					break
				}

				if token, ok := sourceInstruction.Operand.(OptimizerToken); ok {
					value, found := tokens[token.Name]
					if found {
						if value == i.Operand {
							continue
						} else if i.Operand != sourceInstruction.Operand {
							break
						}
					} else {
						token := OptimizerToken{Name: token.Name, Value: i.Operand}
						tokens[token.Name] = token
					}
				}

				if !found {
					break
				}
			}

			if found {
				if count == 0 {
					ui.Debug(ui.ByteCodeLogger, "@@@ Optimizing bytecode %s @@@", b.Name)
				}

				ui.Debug(ui.ByteCodeLogger, "Optimization found in %s: %s", b.Name, optimization.Description)

				// Make a copy of the replacements, with the token values from the
				// source stream inserted as appropriate.
				replacements := []Instruction{}

				for _, replacement := range optimization.Replacement {
					if token, ok := replacement.Operand.(OptimizerToken); ok {
						replacement.Operand = tokens[token.Name].Value
					}

					replacements = append(replacements, replacement)
				}

				b.Patch(idx, len(optimization.Source), replacements)

				count++
			}
		}
	}

	if count > 0 {
		ui.Debug(ui.ByteCodeLogger, "Found %d optimization(s) for net change in size of %d instructions", count, startingSize-b.emitPos)
	}

	return nil
}

func (b *ByteCode) Patch(start, deleteSize int, insert []Instruction) {
	offset := deleteSize - len(insert)

	// Start by deleting the old instructions
	instructions := append(b.instructions[:start], insert...)
	instructions = append(instructions, b.instructions[start+len(insert)+1:]...)

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
