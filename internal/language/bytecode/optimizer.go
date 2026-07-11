package bytecode

import (
	"reflect"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/symbols"
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
// The replacement may be shorter than the pattern (the typical case — the
// optimizer shrinks the bytecode) or the same length.  Growing replacements
// (len(Replacement) > len(Pattern)) are now safe due to the Patch fix
// (OPTIMIZER-5).
type optimization struct {
	// Description is a human-readable label logged when the rule fires.
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
// Performance: the inner rule loop is dispatched through an opcode-indexed
// table (OPTIMIZER-3), so only rules whose first pattern opcode matches the
// current instruction are tried.  Branch-target checking uses a pre-built
// address set (OPTIMIZER-1) rather than a linear scan.  Both improvements
// reduce the per-position cost from O(n*m) to O(p) where p is the average
// number of matching rules per opcode.
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

	// Build two auxiliary structures over the optimizations slice.
	//
	// maxPatternSize: the widest pattern across all enabled rules.  After a
	// successful substitution we back the scanner up by this amount so that
	// the new instructions can participate in further rounds of optimization.
	// The minimum safe retreat is maxPatternSize-1: a pattern that wide starting
	// one instruction before the match could span the replacement.
	//
	// rulesByFirstOpcode: maps the first-instruction opcode of each enabled rule
	// to the indices of rules with that opcode.  At each position we only try
	// the rules whose first opcode matches the current instruction, skipping
	// every other rule without ever examining it (OPTIMIZER-3).
	maxPatternSize := 0
	rulesByFirstOpcode := make(map[Opcode][]int)

	for i, opt := range optimizations {
		if opt.Disable || len(opt.Pattern) == 0 {
			continue
		}

		if n := len(opt.Pattern); n > maxPatternSize {
			maxPatternSize = n
		}

		firstOp := opt.Pattern[0].Operation
		rulesByFirstOpcode[firstOp] = append(rulesByFirstOpcode[firstOp], i)
	}

	// branchTargets is the set of all instruction addresses that are the
	// destination of at least one branch instruction.  We use it in the inner
	// loop to quickly reject patterns that contain a branch target — replacing
	// such a pattern would invalidate any branch pointing into it.
	//
	// The set must be rebuilt after each Patch call because Patch adjusts branch
	// operands.  needsRebuild starts true so we build before the first iteration.
	var branchTargets map[int]bool

	needsRebuild := true

	// Main scan loop.  idx advances forward through the instruction stream.
	// After a successful substitution, idx is backed up so the new instructions
	// are re-evaluated for further optimization opportunities.
	for idx := 0; idx < b.nextAddress; idx++ {
		// Rebuild the branch-target set when the instruction stream changed.
		// This is O(n) but only happens after each Patch, not every iteration
		// (OPTIMIZER-1: pre-built set replaces the old O(n) per-rule scan).
		if needsRebuild {
			branchTargets = make(map[int]bool)

			for _, i := range b.instructions[:b.nextAddress] {
				if i.Operation > BranchInstructions {
					// A branch instruction with a non-integer operand means malformed
					// bytecode.  Rather than aborting the whole pass (the old behavior,
					// OPTIMIZER-8), we simply do not add an entry for it — the
					// branch-target check will then not block any window that happens
					// to overlap this address, which is a conservative safe choice.
					if dest, err := data.Int(i.Operand); err == nil {
						branchTargets[dest] = true
					} else if ui.IsActive(ui.OptimizerLogger) {
						ui.Log(ui.OptimizerLogger, "optimizer.branch.malformed", ui.A{
							"operand": i.Operand})
					}
				}
			}

			needsRebuild = false
		}

		// Use the opcode dispatch table to find only the rules whose first
		// pattern instruction matches the opcode at the current position.
		// Rules with a different first opcode cannot possibly match here.
		candidates := rulesByFirstOpcode[b.instructions[idx].Operation]

		for _, ruleIdx := range candidates {
			optimization := optimizations[ruleIdx]

			// operandValues maps placeholder names to the concrete operand values
			// captured while matching the current pattern.  Used to fill in
			// replacement operands at emit time.
			operandValues := map[string]placeholder{}

			// registers is a small scratch array for accumulating computed values
			// (e.g., summing sequential PopScope counts via OptCount).
			registers := make([]any, 5)

			// Assume this rule matches until we find evidence otherwise.
			found := true

			// Safety check: if any branch instruction targets an address inside the
			// candidate pattern window, we must not replace those instructions —
			// doing so would invalidate the branch.
			//
			// With the pre-built branchTargets set this is O(patternLen) rather
			// than the former O(n) full-scan (OPTIMIZER-1).
			for offset := 0; offset < len(optimization.Pattern); offset++ {
				if branchTargets[idx+offset] {
					found = false

					break
				}
			}

			if !found {
				continue
			}

			// Match each instruction in the pattern against the real instruction
			// at the corresponding offset from idx.
			for sourceIdx, sourceInstruction := range optimization.Pattern {
				// If the remaining bytecode is shorter than the rest of the
				// pattern, this rule cannot match — skip it.
				if b.nextAddress <= idx+sourceIdx {
					found = false

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

				// Check equality between the pattern operand and the real operand.
				// operandEqual uses a type-switch for common types to avoid
				// reflection overhead, with reflect.DeepEqual as a fallback
				// (OPTIMIZER-2).
				if operandEqual(sourceInstruction.Operand, i.Operand) {
					found = true

					continue
				}

				// operandEqual returned false.  Determine what to do:
				//
				//  • placeholder{}: fall through to the capture/consistency
				//    block below — its semantics are handled there.
				//
				//  • empty{}: the "operand must be nil" check at the top of
				//    this loop already set found=false when i.Operand != nil.
				//    If we reach here with empty{}, i.Operand IS nil and the
				//    nil-operand match was already accepted; operandEqual just
				//    can't compare empty{} to nil directly, so we fall through.
				//
				//  • any other concrete source value: the operands genuinely
				//    don't match — reject the rule immediately.
				_, isPlaceholder := sourceInstruction.Operand.(placeholder)
				_, isEmpty := sourceInstruction.Operand.(empty)

				if !isPlaceholder && !isEmpty {
					found = false

					break
				}

				// Handle placeholder operands: capture or verify consistency.
				if token, ok := sourceInstruction.Operand.(placeholder); ok {
					value, inMap := operandValues[token.Name]

					if inMap {
						// This placeholder name appeared earlier in the pattern.
						// All occurrences must bind to the same operand value.
						// If they differ, the match fails.  Use break not continue
						// so we stop checking once a mismatch is found (OPTIMIZER-6,
						// OPTIMIZER-9).
						if value.Value != i.Operand {
							found = false

							break
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

				if ui.IsActive(ui.OptimizerLogger) {
					ui.Log(ui.OptimizerLogger, "optimizer.found", ui.A{
						"name": b.name,
						"desc": optimization.Description})
				}

				// Build the concrete replacement instruction list by resolving
				// any placeholder or register references in the Replacement template.
				replacements := []instruction{}

				for _, replacement := range optimization.Replacement {
					newInstruction := replacement

					if token, ok := replacement.Operand.(placeholder); ok {
						switch token.Operation {
						case optRunConstantFragment:
							// Constant folding: pre-compute the result of the matched
							// instruction sequence.  A fast arithmetic path is tried
							// first to avoid the full interpreter overhead (OPTIMIZER-7);
							// it falls back to executeFragment for non-numeric types.
							patLen := len(optimization.Pattern)
							v1 := b.instructions[idx].Operand
							v2 := b.instructions[idx+1].Operand
							arithOp := b.instructions[idx+patLen-1].Operation

							if result, ok := tryConstantArithmetic(arithOp, v1, v2); ok {
								newInstruction.Operand = result
							} else {
								v, err := b.executeFragment(idx, idx+patLen)
								if err != nil {
									return 0, err
								}

								newInstruction.Operand = v
							}

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
				// adjusted automatically by Patch.
				b.Patch(idx, len(optimization.Pattern), replacements)

				// Mark the branch-target set as stale: Patch has adjusted branch
				// operands throughout the instruction stream, so branchTargets no
				// longer reflects the current state (OPTIMIZER-1).
				needsRebuild = true

				// Back up the scanner so newly-emitted instructions can participate
				// in further optimizations.  We retreat by maxPatternSize-1 positions
				// before the current idx: that is the furthest-back position at which
				// any rule (up to maxPatternSize instructions wide) could start and
				// still include the new replacement instruction at idx.  Retreating
				// further would be unnecessary work; retreating less would miss some
				// opportunities (OPTIMIZER-4).
				idx -= maxPatternSize - 1
				if idx < -1 {
					idx = -1 // after the loop's idx++, resumes at 0
				}

				count++

				// Stop trying further rules at the old position; the outer loop
				// will now restart from the backed-up position with all rules
				// freshly available (OPTIMIZER-3 interaction: correct dispatch
				// requires reading the new opcode at the backed-up position).
				break
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

// operandEqual compares two instruction operands for equality using a
// type-switch for the most common types to avoid reflection overhead
// (OPTIMIZER-2).  reflect.DeepEqual is used as a fallback for types not
// enumerated here (e.g. StackMarker, which contains a []any field and
// is therefore not directly comparable with ==).
func operandEqual(a, b any) bool {
	switch av := a.(type) {
	case int:
		bv, ok := b.(int)

		return ok && av == bv

	case int64:
		bv, ok := b.(int64)

		return ok && av == bv

	case float64:
		bv, ok := b.(float64)

		return ok && av == bv

	case bool:
		bv, ok := b.(bool)

		return ok && av == bv

	case string:
		bv, ok := b.(string)

		return ok && av == bv

	case nil:
		return b == nil

	default:
		// Handles StackMarker (unexported []any field prevents ==) and any
		// other composite types that appear as concrete operands.
		return reflect.DeepEqual(a, b)
	}
}

// tryConstantArithmetic attempts to evaluate op(v1, v2) directly for common
// numeric and string types without running the interpreter (OPTIMIZER-7).
//
// Returns (result, true) when the computation succeeds.
// Returns (nil, false) when the types are not handled here; the caller should
// fall back to executeFragment for the full interpreter path.
func tryConstantArithmetic(op Opcode, v1, v2 any) (any, bool) {
	var err error

	// The operands may be wrapped in a constant instruction; unwrap them to get
	// the actual values.
	v1 = data.UnwrapConstant(v1)
	v2 = data.UnwrapConstant(v2)

	// Handle string concatenation as a special case since it is the only valid
	// operation on strings.  If the first operand is a string and the operator is
	// Add, we require the second operand to also be a string and concatenate
	// them.  For any other operator or if the first operand is a string but the
	// second is not, we cannot fold this pattern — return false so the caller
	// will fall back to executeFragment, which can handle more complex cases
	// like string repetition.
	if s1, ok := v1.(string); ok {
		if op == Add {
			if s2, ok := v2.(string); ok {
				return s1 + s2, true
			}
		}

		return nil, false
	}

	// For non-string types, we only handle numeric operations here.  If either
	// operand is not numeric, return false so the caller will fall back to
	// executeFragment, which can handle more complex cases like type aliases.
	if !data.IsNumeric(v1) || !data.IsNumeric(v2) {
		return nil, false
	}

	// This is compile-time constant folding: both v1 and v2 are always
	// literal constants here by construction, so v1Const == v2Const is always
	// true and Normalize's constant-adaptation branch never fires; this
	// falls straight through to the unchanged kind-ordering promotion, and
	// the strict argument is therefore moot (no type-strictness context is
	// available at this compile-time-only call site).
	v1, v2, err = data.Normalize(v1, true, v2, true, false)
	if err != nil {
		return nil, false
	}

	switch a := v1.(type) {
	case int:
		b, ok := v2.(int)
		if !ok {
			return nil, false
		}

		switch op {
		case Add:
			return a + b, true
		case Sub:
			return a - b, true
		case Mul:
			return a * b, true
		}

	case int64:
		b, ok := v2.(int64)
		if !ok {
			return nil, false
		}

		switch op {
		case Add:
			return a + b, true
		case Sub:
			return a - b, true
		case Mul:
			return a * b, true
		}

	case float64:
		b, ok := v2.(float64)
		if !ok {
			return nil, false
		}

		switch op {
		case Add:
			return a + b, true
		case Sub:
			return a - b, true
		case Mul:
			return a * b, true
		case Div:
			return a / b, true
		}

	case string:
		// Only addition (concatenation) applies to strings.
		if op == Add {
			b, ok := v2.(string)
			if ok {
				return a + b, true
			}
		}
	}

	return nil, false
}

// executeFragment runs a short slice of bytecode instructions as an isolated
// mini-program and returns the single value left on the evaluation stack.
//
// It is used as the fallback path for constant-folding (optRunConstantFragment)
// when tryConstantArithmetic cannot handle the operand types (e.g., type aliases
// or string repetition).  The fast path in tryConstantArithmetic handles the
// common int/float64/string cases without the overhead of building a full
// interpreter context (OPTIMIZER-7).
//
// The instructions from index start (inclusive) up to end (exclusive) are
// copied into a fresh ByteCode object, a Stop instruction is appended, and the
// code is executed with strict type enforcement in an empty symbol table.
func (b *ByteCode) executeFragment(start, end int) (any, error) {
	fragment := New("code fragment")

	for idx := start; idx < end; idx++ {
		i := b.instructions[idx]
		fragment.Emit(i.Operation, i.Operand)
	}

	fragment.Emit(Stop)

	s := symbols.NewSymbolTable("fragment")
	c := NewContext(s, fragment)
	c.typeStrictness = defs.StrictTypeEnforcement

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
// strictly greater than start is adjusted by (deleteSize - len(insert)) so
// that it still points to the same logical instruction despite the shift.
//
// The tail of the instruction array is explicitly copied before the splice
// so that growing replacements (len(insert) > deleteSize) do not corrupt the
// tail — the former append-chain was only safe when the replacement was no
// larger than the deleted region (OPTIMIZER-5).
func (b *ByteCode) Patch(start, deleteSize int, insert []instruction) {
	// offset is positive when shrinking (more deleted than inserted),
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

	// Save the tail independently BEFORE any appending that might modify the
	// backing array.  Without this copy, a growing replacement (len(insert) >
	// deleteSize) would overwrite the tail before we appended it (OPTIMIZER-5).
	tailStart := start + deleteSize
	tail := make([]instruction, b.nextAddress-tailStart)
	copy(tail, b.instructions[tailStart:b.nextAddress])

	// Build the new instruction slice: head + insert + tail.
	newLen := start + len(insert) + len(tail)
	instructions := make([]instruction, 0, newLen)
	instructions = append(instructions, b.instructions[:start]...)
	instructions = append(instructions, insert...)
	instructions = append(instructions, tail...)

	// Fix up branch targets that pointed after the patched region.
	// Targets at or before 'start' are unaffected.
	// Targets after 'start' shifted by -offset because the total
	// instruction count changed.
	for i := 0; i < len(instructions); i++ {
		if instructions[i].Operation > BranchInstructions {
			destination, err := data.Int(instructions[i].Operand)
			if err != nil && ui.IsActive(ui.OptimizerLogger) {
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
