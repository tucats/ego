package bytecode

import (
	"reflect"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

// optimizerOperation describes what action a placeholder should perform when it
// is encountered during pattern matching or replacement generation.
type optimizerOperation int

const (
	// optNothing means the placeholder carries no special action; its matched
	// value is simply captured by name and replayed into the replacement stream.
	optNothing optimizerOperation = iota

	// optStore saves the matched operand into a numbered register.  Registers
	// are small scratch slots (registers[0..4]) used when the replacement needs
	// a value that was computed from the matched sequence, not just copied from it.
	optStore

	// optRead reads a value from a numbered register and uses it as the
	// replacement instruction's operand.
	optRead

	// OptCount accumulates operand values into a register.  Each time the
	// pattern sees a new instruction with this placeholder, the operand (treated
	// as an integer, defaulting to 1 when nil) is ADDED to the register.
	// Used to merge multiple sequential PopScope instructions into one.
	OptCount

	// optRunConstantFragment signals that the replacement operand should be
	// computed by actually running the matched instructions as a mini-program
	// and capturing the result.  This is how constant folding works: the
	// optimizer executes the original Push/Push/Add sequence and stores the
	// pre-computed result as the new operand for a single Push.
	optRunConstantFragment
)

// empty is a sentinel type used as an instruction operand in patterns.
// When an optimization pattern specifies Operand: empty{}, it means the
// real instruction must have a nil operand — it carries no data.
// This lets patterns distinguish "I don't care about the operand" (use a
// placeholder{}) from "the operand must be absent" (use empty{}).
type empty struct{}

// placeholder is used inside optimization patterns and replacements as a
// stand-in for a real operand value that won't be known until matching time.
//
// In a PATTERN instruction, a placeholder says "this operand can be anything;
// capture it under the given Name so the replacement can use it."  If the same
// Name appears more than once in a pattern, all occurrences must have the same
// value (consistency check).
//
// In a REPLACEMENT instruction, a placeholder says "fill in the operand from
// the captured value stored under Name" — or, when Operation is optRead or
// optRunConstantFragment, derive the operand value from a register or by
// running a code fragment.
type placeholder struct {
	// Name is the key used to store and recall matched operand values across
	// the pattern→replacement boundary.
	Name string

	// MustBeString, when true, requires that the matched operand be a Go string.
	// The optimizer uses this to prevent folding patterns where the operand
	// could be any type (e.g., collapsing a Push+StoreAlways only when the
	// variable name is definitely a string).
	MustBeString bool

	// Operation controls any side effect performed when this placeholder is
	// encountered: optNothing (default), optStore, OptCount, optRead, or
	// optRunConstantFragment.
	Operation optimizerOperation

	// Register is the scratch-slot index (0–4) used by optStore, OptCount,
	// and optRead.
	Register int

	// Value is only populated after matching; it holds the concrete operand
	// captured from the instruction stream.  It is zero inside pattern
	// definitions.
	Value any
}

// optimization describes a single peephole rule: a Pattern of consecutive
// instructions to look for, and a Replacement sequence to substitute when the
// pattern is matched.
//
// The replacement may be shorter than the pattern (which is the typical case —
// the optimizer is shrinking the bytecode) or the same length.  The optimizer
// currently does not support replacements that are longer than the pattern
// (see OPTIMIZER-5 in docs/BYTECODE_ISSUES.md).
type optimization struct {
	// Description is a human-readable label logged when the optimization fires.
	Description string

	// Debug, when true, enables extra diagnostic output for this specific rule
	// during development.  Leave false in production rules.
	Debug bool

	// Disable, when true, skips this rule entirely.  Useful for temporarily
	// turning off a rule while investigating interactions between rules without
	// removing the definition.
	Disable bool

	// Pattern is the sequence of instructions to match.  Operands may be
	// concrete values, empty{} (nil required), or placeholder{} (any value,
	// with optional capture/consistency semantics).
	Pattern []instruction

	// Replacement is the sequence of instructions to emit in place of the
	// matched pattern.  An empty replacement removes the matched instructions
	// entirely.  Operands may reference captured placeholder values.
	Replacement []instruction
}

// optimize runs a peephole optimizer over the bytecode sequence stored in b.
//
// A peephole optimizer works by sliding a small window over the instruction
// stream and replacing known inefficient patterns with more efficient ones.
// This is called "peephole" because it only looks at a small region at a time,
// not the full program.
//
// The parameter count tracks how many substitutions have been made across
// repeated calls; pass 0 on the first call.  It is returned incremented by
// however many substitutions were made during this call.
//
// The function returns early (count, nil) if the bytecode has already been
// optimized and not modified since.
//
// Performance note: the dominant cost is the branch-target scan performed for
// every (position, optimization) pair — O(n * m * n) in the instruction count n
// and optimization count m.  See OPTIMIZER-1 in docs/BYTECODE_ISSUES.md for a
// proposed improvement.
func (b *ByteCode) optimize(count int) (int, error) {
	// If nothing has changed since the last optimization pass, bail out early.
	// The 'optimized' flag is cleared whenever an instruction is emitted or
	// modified, and set at the very end of this function.
	if b.optimized {
		return 0, nil
	}

	// Remember the original size so we can report how many instructions were
	// eliminated when logging is active.
	startingSize := b.nextAddress

	// Pre-compute the largest pattern size among all active rules.  After a
	// successful substitution we back the scanner up by this amount so that
	// the new instructions are re-evaluated — a substitution may reveal a
	// further opportunity that spans the boundary between old and new code.
	maxPatternSize := 0
	for _, optimization := range optimizations {
		if optPatternSize := len(optimization.Pattern); optPatternSize > maxPatternSize {
			maxPatternSize = optPatternSize
		}
	}

	// Main scan loop.  idx advances forward through the instruction stream.
	// After a successful substitution, idx is backed up (see below) so the
	// new instructions are reconsidered.
	for idx := 0; idx < b.nextAddress; idx++ {
		found := false

		// Try every optimization rule at the current position.
		for _, optimization := range optimizations {
			if optimization.Disable {
				continue
			}

			// operandValues maps placeholder names to the concrete operand values
			// captured while matching the current pattern.  Used to fill in
			// replacement operands at emit time.
			operandValues := map[string]placeholder{}

			// registers is a small scratch array for accumulating computed values
			// (e.g., summing sequential PopScope counts via OptCount).
			registers := make([]any, 5)

			// Assume the pattern matches until we find evidence otherwise.
			found = true

			// Safety check: reject this position if any branch instruction in the
			// ENTIRE bytecode has a destination inside the candidate pattern region.
			// Patching away instructions that are branch targets would invalidate
			// those branches.
			//
			// NOTE: this scan is O(n) per (position, optimization) pair, making the
			// overall optimizer O(n^2 * m).  See OPTIMIZER-1 in BYTECODE_ISSUES.md
			// for a proposed fix using a pre-built branch-target set.
			for _, i := range b.instructions {
				if i.Operation > BranchInstructions {
					destination, err := data.Int(i.Operand)
					if err != nil {
						// A branch instruction with a non-integer operand indicates
						// malformed bytecode.  Aborting the entire optimization pass
						// is conservative but safe; see OPTIMIZER-8 in
						// BYTECODE_ISSUES.md for a proposed softer approach.
						return 0, errors.New(err)
					}

					// If the branch lands anywhere inside the candidate window,
					// we cannot safely remove or rearrange those instructions.
					if destination >= idx && destination < idx+len(optimization.Pattern) {
						found = false

						break
					}
				}
			}

			if !found {
				continue
			}

			// Match each instruction in the pattern against the real instruction
			// at the corresponding offset from idx.
			for sourceIdx, sourceInstruction := range optimization.Pattern {
				// If the remaining bytecode is shorter than the rest of the pattern,
				// there is nothing to match — skip this rule.
				if b.nextAddress <= idx+sourceIdx {
					found = false

					// Stop checking this pattern; try the next one.
					break
				}

				// The real instruction we are comparing against.
				i := b.instructions[idx+sourceIdx]

				// empty{} in the pattern means "operand must be nil".
				if _, isEmpty := sourceInstruction.Operand.(empty); isEmpty && i.Operand != nil {
					found = false

					break
				}

				// The opcodes must match exactly.
				if sourceInstruction.Operation != i.Operation {
					found = false

					break
				}

				// If the pattern has a placeholder with MustBeString, the real
				// operand must be a Go string.  This guards rules like the
				// Push+StoreAlways collapse, which only applies when the variable
				// name is definitely a string (not some other type).
				if pattern, ok := sourceInstruction.Operand.(placeholder); ok {
					if pattern.MustBeString {
						if _, ok := i.Operand.(string); !ok {
							found = false

							break
						}
					}
				}

				// Fast path for string operands: compare directly without going
				// through reflect.DeepEqual.
				if operand1, ok := sourceInstruction.Operand.(string); ok {
					if operand2, ok := i.Operand.(string); ok {
						found = operand1 == operand2

						continue
					}
				}

				// General equality check for concrete (non-placeholder) operands.
				// reflect.DeepEqual handles all types uniformly but is expensive;
				// the fast string path above avoids it for the common string case.
				// See OPTIMIZER-2 in BYTECODE_ISSUES.md for a proposed broader
				// type-switch approach.
				if reflect.DeepEqual(sourceInstruction.Operand, i.Operand) {
					found = true

					continue
				}

				// Handle placeholder operands: capture or verify consistency.
				if token, ok := sourceInstruction.Operand.(placeholder); ok {
					value, inMap := operandValues[token.Name]

					if inMap {
						// This placeholder name appeared earlier in the pattern.
						// All occurrences must bind to the same operand value.
						if value.Value == i.Operand {
							// Values agree — nothing to do, keep matching.
						} else if i.Operand != sourceInstruction.Operand {
							// The operand differs from what we captured the first
							// time we saw this name.  The pattern does not match.
							// Note: the `else if` condition is always true for real
							// bytecode (real operands are never placeholder structs);
							// see OPTIMIZER-9 in BYTECODE_ISSUES.md for a cleanup
							// suggestion.
							found = false

							continue
						}
					} else {
						// First occurrence of this placeholder name.  Record the
						// operand value and, if the placeholder has a register
						// operation, update the register.
						switch token.Operation {
						case OptCount:
							// Accumulate: add the operand (or 1 if nil) to the register.
							// Used for merging consecutive PopScope instructions.
							increment := 1
							if i.Operand != nil {
								increment, _ = data.Int(i.Operand)
							}

							registers[token.Register] = data.IntOrZero(registers[token.Register]) + increment

						case optStore:
							// Save the raw operand value into the register for
							// later retrieval with optRead.
							registers[token.Register] = i.Operand
						}

						operandValues[token.Name] = placeholder{Name: token.Name, Value: i.Operand}
					}
				}
			}

			// If every instruction in the pattern matched, apply the substitution.
			if found {
				// Log the first match for a given bytecode object (count==0) so the
				// developer can see that the optimizer is active.
				if count == 0 && ui.IsActive(ui.OptimizerLogger) {
					ui.Log(ui.OptimizerLogger, "optimizer.bytecode", ui.A{
						"name": b.name})
				}

				ui.Log(ui.OptimizerLogger, "optimizer.found", ui.A{
					"name": b.name,
					"desc": optimization.Description})

				// Build the concrete replacement instruction list by resolving
				// any placeholder or register references in the Replacement template.
				replacements := []instruction{}

				for _, replacement := range optimization.Replacement {
					newInstruction := replacement

					if token, ok := replacement.Operand.(placeholder); ok {
						switch token.Operation {
						case optRunConstantFragment:
							// Execute the matched instructions as a tiny program and
							// use the result as the replacement operand.  This is the
							// constant-folding path: Push(2)+Push(3)+Add → Push(5).
							// See OPTIMIZER-7 for the overhead concern.
							v, err := b.executeFragment(idx, idx+len(optimization.Pattern))
							if err != nil {
								return 0, err
							}

							newInstruction.Operand = v

						case optRead:
							// Retrieve a value computed by OptCount or optStore during
							// the match phase.
							newInstruction.Operand = registers[token.Register]

						default:
							// Simple name-based capture: fill in the operand that was
							// recorded when this placeholder name was first matched.
							newInstruction.Operand = operandValues[token.Name].Value
						}
					} else {
						// The replacement operand may itself be a slice of items, some
						// of which are placeholders.  Resolve each element individually.
						if tokenArray, ok := replacement.Operand.([]any); ok {
							newArray := []any{}

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

				// Splice the replacement instructions into the bytecode in place of
				// the matched pattern.  Branch targets throughout the code are
				// adjusted automatically.
				b.Patch(idx, len(optimization.Pattern), replacements)

				// Back up the scanner so the newly-emitted instructions can
				// participate in further optimizations.  We retreat by
				// maxPatternSize (the widest pattern across all rules) to be safe,
				// though retreating by only the current pattern's size would
				// suffice — see OPTIMIZER-4 in BYTECODE_ISSUES.md.
				idx = (idx - maxPatternSize) - 1

				if idx < 0 {
					idx = 0
				}

				count++
			}
		}
	}

	// Log a summary if any substitutions were made and the bytecode shrank.
	if count > 0 && ui.IsActive(ui.OptimizerLogger) && b.nextAddress != startingSize {
		ui.Log(ui.OptimizerLogger, "optimizer.stats", ui.A{
			"count":  count,
			"change": startingSize - b.nextAddress})
	}

	b.optimized = true

	return count, nil
}

// executeFragment runs a short slice of bytecode instructions as an isolated
// mini-program and returns the single value left on the evaluation stack.
//
// It is used by the constant-folding optimizations (optRunConstantFragment) to
// pre-compute the result of sequences like Push(2)+Push(3)+Add at compile time,
// replacing them with a single Push(5).
//
// The instructions from index start (inclusive) up to end (exclusive) are
// copied into a fresh ByteCode object, a Stop instruction is appended, and the
// code is executed with strict type enforcement in an empty symbol table.
//
// Performance note: this creates a full ByteCode + SymbolTable + Context object
// for every constant fold, even for trivially simple arithmetic.  See OPTIMIZER-7
// in BYTECODE_ISSUES.md for a proposal to replace this with direct computation.
func (b *ByteCode) executeFragment(start, end int) (any, error) {
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

// Patch replaces a contiguous run of instructions with a new sequence.
//
// start is the index of the first instruction to replace.
// deleteSize is how many instructions to remove starting at start.
// insert is the sequence of instructions to splice in at that position.
//
// After splicing, every branch instruction whose destination address is
// strictly greater than start is adjusted by the difference
// (deleteSize - len(insert)) so that it still points to the same logical
// instruction despite the shift.
//
// IMPORTANT: the current implementation is only safe when len(insert) <=
// deleteSize (the replacement shrinks or keeps the same number of instructions).
// If insert is longer than deleteSize the append chain will corrupt the tail of
// the instruction array before it is captured.  All current optimizer rules only
// shrink the bytecode, so this is a latent issue rather than an active bug.
// See OPTIMIZER-5 in docs/BYTECODE_ISSUES.md.
func (b *ByteCode) Patch(start, deleteSize int, insert []instruction) {
	// offset is positive when we are shrinking (more deleted than inserted),
	// zero when same size, and negative when growing.  Branch destinations
	// that follow the patched region are decremented by this amount.
	offset := deleteSize - len(insert)
	savedBytecodeLoggerState := ui.IsActive(ui.ByteCodeLogger)

	// Restore the ByteCode disassembly logger state when we're done, in case
	// we temporarily enabled it for optimizer diagnostics below.
	defer func() {
		ui.Active(ui.ByteCodeLogger, savedBytecodeLoggerState)
	}()

	// When optimizer logging is active, disassemble the region before and
	// after the patch so the developer can see exactly what changed.
	if ui.IsActive(ui.OptimizerLogger) {
		ui.Active(ui.ByteCodeLogger, true)
		ui.Log(ui.OptimizerLogger, "optimizer.existing.code", nil)
		b.Disasm(start, start+deleteSize)
	}

	// Splice: cut out the old instructions and insert the new ones.
	// Step 1: truncate at 'start' and append the replacement.
	// Step 2: append the original tail that follows the deleted region.
	instructions := append(b.instructions[:start], insert...)
	instructions = append(instructions, b.instructions[start+deleteSize:]...)

	// Fix up branch targets that pointed after the patched region.
	// Targets at or before 'start' are unaffected (the instructions they
	// point to did not move).  Targets after 'start' shifted by -offset
	// because the total instruction count changed.
	for i := 0; i < len(instructions); i++ {
		if instructions[i].Operation > BranchInstructions {
			destination, err := data.Int(instructions[i].Operand)
			if err != nil {
				ui.Log(ui.OptimizerLogger, "optimizer.dest.error", ui.A{
					"error": err})
			}

			if destination > start {
				instructions[i].Operand = destination - offset
			}
		}
	}

	b.instructions = instructions
	b.nextAddress = b.nextAddress - offset

	if ui.IsActive(ui.OptimizerLogger) {
		ui.Active(ui.ByteCodeLogger, true)
		ui.Log(ui.OptimizerLogger, "optimizer.new.code", nil)
		b.Disasm(start, start+len(insert))
	}
}
