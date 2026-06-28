package bytecode

// optimizations is the master list of peephole rules that the optimizer applies.
//
// Each entry is an optimization struct with:
//   - Description: a short human-readable label logged when the rule fires.
//   - Pattern:     the sequence of instructions to look for.
//   - Replacement: the cheaper sequence to substitute (may be empty to delete).
//
// Rules are tried in order at each bytecode position.  When a rule fires the
// scanner backs up by maxPatternSize positions so that the freshly emitted
// instructions can themselves participate in further rounds of optimization.
//
// Operand conventions inside Pattern and Replacement:
//   - A concrete value (e.g., "name", 1) matches/emits that exact operand.
//   - empty{}           matches an instruction whose operand is nil.
//   - placeholder{Name} captures any operand under the given name and replays
//                       it into the Replacement via the same placeholder{Name}.
//   - placeholder{Name, MustBeString: true} only matches when the operand is
//                       a Go string.
//   - placeholder{Operation: optRunConstantFragment} in a Replacement causes the
//                       matched instructions to be executed and the result used
//                       as the replacement operand (constant folding).
//   - placeholder{Operation: OptCount, Register: N} accumulates integer operands
//                       into register N (for collapsing sequential PopScope calls).
//   - placeholder{Operation: optRead, Register: N} reads register N as the
//                       replacement operand.
var optimizations = []optimization{
	// ---- Dead-assignment elimination ----------------------------------------

	{
		// An assignment that pushes a marker and then immediately drops to that
		// same marker produces no net effect.  Remove both instructions.
		//
		// Pattern:
		//   Push  StackMarker("let")
		//   DropToMarker StackMarker("let")
		//
		// This is generated when the compiler emits a "let" sequence for an
		// expression statement whose value is never used.
		Description: "Assignment optimized away",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   NewStackMarker("let"),
			},
			{
				Operation: DropToMarker,
				Operand:   NewStackMarker("let"),
			},
		},
		// Empty Replacement means both instructions are deleted.
	},
	{
		// Pushing a constant and then immediately dropping it (Drop 1) has no
		// observable effect.  Delete both instructions.
		//
		// Pattern:
		//   Push  <any value>
		//   Drop  1
		Description: "Write constant to null variable",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{},
			},
			{
				Operation: Drop,
				Operand:   1,
			},
		},
		Replacement: []instruction{},
	},

	// ---- Blank identifier (_) optimizations ---------------------------------

	{
		// Storing to the blank identifier "_" is a no-op; replace the Store with
		// a plain Drop so the value is discarded without touching the symbol table.
		//
		// Pattern:
		//   Store  "_"
		//
		// Replacement:
		//   Drop
		Description: "Store to null variable",
		Pattern: []instruction{
			{
				Operation: Store,
				Operand:   "_",
			},
		},
		Replacement: []instruction{
			{
				Operation: Drop,
			},
		},
	},
	{
		// Creating the blank identifier "_" via SymbolOptCreate is a no-op
		// (there is nothing to create).  Delete the instruction entirely.
		//
		// Pattern:
		//   SymbolOptCreate  "_"
		Description: "Create null variable",
		Pattern: []instruction{
			{
				Operation: SymbolOptCreate,
				Operand:   "_",
			},
		},
		Replacement: []instruction{},
	},

	// ---- Increment fusion ---------------------------------------------------

	{
		// A load-add-store sequence on the same variable with a constant step is
		// fused into a single Increment instruction, which avoids two symbol-table
		// lookups and intermediate stack traffic.
		//
		// Pattern:
		//   Load   <name>
		//   Push   <increment>
		//   Add
		//   Store  <name>           ← must be the same name as the Load
		//
		// Replacement:
		//   Increment  [<name>, <increment>]
		Description: "Constant increment",
		Pattern: []instruction{
			{
				Operation: Load,
				Operand:   placeholder{Name: "name"},
			},
			{
				Operation: Push,
				Operand:   placeholder{Name: "increment"},
			},
			{
				Operation: Add,
			},
			{
				Operation: Store,
				Operand:   placeholder{Name: "name"},
			},
		},
		Replacement: []instruction{
			{
				Operation: Increment,
				Operand: []any{
					placeholder{Name: "name"},
					placeholder{Name: "increment"},
				},
			},
		},
	},

	// ---- Comparison with constant value -------------------------------------
	//
	// For each comparison opcode (LessThan, LessThanOrEqual, GreaterThan,
	// GreaterThanOrEqual, Equal, NotEqual), a preceding Push of a constant
	// can be folded into the comparison instruction's operand.  The resulting
	// single instruction pops only the left-hand side from the stack and
	// compares it directly against the embedded constant, eliminating one stack
	// push/pop per comparison.

	{
		// Pattern:
		//   Push  <value>
		//   LessThan (no operand)
		//
		// Replacement:
		//   LessThan  [<value>]
		Description: "Less than constant value",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "value"},
			},
			{
				Operation: LessThan,
				Operand:   empty{},
			},
		},
		Replacement: []instruction{
			{
				Operation: LessThan,
				Operand:   []any{placeholder{Name: "value"}},
			},
		},
	},
	{
		// Pattern:
		//   Push  <value>
		//   LessThanOrEqual (no operand)
		//
		// Replacement:
		//   LessThanOrEqual  [<value>]
		Description: "Less than or equal to constant value",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "value"},
			},
			{
				Operation: LessThanOrEqual,
				Operand:   empty{},
			},
		},
		Replacement: []instruction{
			{
				Operation: LessThanOrEqual,
				Operand:   []any{placeholder{Name: "value"}},
			},
		},
	},
	{
		// Pattern:
		//   Push  <value>
		//   GreaterThan (no operand)
		//
		// Replacement:
		//   GreaterThan  [<value>]
		Description: "Greater than constant value",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "value"},
			},
			{
				Operation: GreaterThan,
				Operand:   empty{},
			},
		},
		Replacement: []instruction{
			{
				Operation: GreaterThan,
				Operand:   []any{placeholder{Name: "value"}},
			},
		},
	},
	{
		// Pattern:
		//   Push  <value>
		//   GreaterThanOrEqual (no operand)
		//
		// Replacement:
		//   GreaterThanOrEqual  [<value>]
		Description: "Greater than or equal to constant value",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "value"},
			},
			{
				Operation: GreaterThanOrEqual,
				Operand:   empty{},
			},
		},
		Replacement: []instruction{
			{
				Operation: GreaterThanOrEqual,
				Operand:   []any{placeholder{Name: "value"}},
			},
		},
	},
	{
		// Pattern:
		//   Push  <value>
		//   Equal (no operand)
		//
		// Replacement:
		//   Equal  [<value>]
		Description: "Equal to constant value",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "value"},
			},
			{
				Operation: Equal,
				Operand:   empty{},
			},
		},
		Replacement: []instruction{
			{
				Operation: Equal,
				Operand:   []any{placeholder{Name: "value"}},
			},
		},
	},
	{
		// Pattern:
		//   Push  <value>
		//   NotEqual (no operand)
		//
		// Replacement:
		//   NotEqual  [<value>]
		Description: "Not equal to constant value",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "value"},
			},
			{
				Operation: NotEqual,
				Operand:   empty{},
			},
		},
		Replacement: []instruction{
			{
				Operation: NotEqual,
				Operand:   []any{placeholder{Name: "value"}},
			},
		},
	},

	// ---- Debug-info simplification ------------------------------------------

	{
		// Two consecutive AtLine instructions: only the second (later) one matters.
		// The first is redundant — drop it.
		//
		// Pattern:
		//   AtLine  <line1>
		//   AtLine  <line2>
		//
		// Replacement:
		//   AtLine  <line2>
		Description: "Sequential AtLine opcodes",
		Pattern: []instruction{
			{
				Operation: AtLine,
				Operand:   placeholder{Name: "line1"},
			},
			{
				Operation: AtLine,
				Operand:   placeholder{Name: "line2"},
			},
		},
		Replacement: []instruction{
			{
				Operation: AtLine,
				Operand:   placeholder{Name: "line2"},
			},
		},
	},

	// ---- Receiver loading ---------------------------------------------------

	{
		// Load followed immediately by SetThis can be fused into a single
		// LoadThis, which both loads the value and sets it as the method receiver
		// in one operation.
		//
		// Pattern:
		//   Load     <name>
		//   SetThis  (no operand)
		//
		// Replacement:
		//   LoadThis  <name>
		Description: "Load followed by SetThis",
		Pattern: []instruction{
			{
				Operation: Load,
				Operand:   placeholder{Name: "name"},
			},
			{
				Operation: SetThis,
				Operand:   empty{},
			},
		},
		Replacement: []instruction{
			{
				Operation: LoadThis,
				Operand:   placeholder{Name: "name"},
			},
		},
	},

	// ---- Store fusion -------------------------------------------------------

	{
		// A Push of a constant value immediately before CreateAndStore can be
		// collapsed: the constant is embedded as a second operand of CreateAndStore,
		// removing the intermediate stack traffic.
		//
		// Pattern:
		//   Push           <value>
		//   CreateAndStore <name>
		//
		// Replacement:
		//   CreateAndStore  [<name>, <value>]
		Description: "Collapse constant Push and CreateAndStore",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "value"},
			},
			{
				Operation: CreateAndStore,
				Operand:   placeholder{Name: "name"},
			},
		},
		Replacement: []instruction{
			{
				Operation: CreateAndStore,
				Operand: []any{
					placeholder{Name: "name"},
					placeholder{Name: "value"},
				},
			},
		},
	},
	{
		// The compiler wraps variable declarations in a Push(marker)+…+DropToMarker
		// "let" frame to allow unwinding on error.  When the body of that frame is
		// a single constant Push+CreateAndStore, the marker frame is unnecessary
		// and can be removed.
		//
		// Pattern:
		//   Push           StackMarker("let")
		//   Push           <constant>
		//   CreateAndStore <name>
		//   DropToMarker   StackMarker("let")
		//
		// Replacement:
		//   Push           <constant>
		//   CreateAndStore <name>
		Description: "Unnecessary stack marker for constant store",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   NewStackMarker("let"),
			},
			{
				Operation: Push,
				Operand:   placeholder{Name: "constant"},
			},
			{
				Operation: CreateAndStore,
				Operand:   placeholder{Name: "name"},
			},
			{
				Operation: DropToMarker,
				Operand:   NewStackMarker("let"),
			},
		},
		Replacement: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "constant"},
			},
			{
				Operation: CreateAndStore,
				Operand:   placeholder{Name: "name"},
			},
		},
	},

	// ---- Scope management ---------------------------------------------------

	{
		// Two consecutive PopScope instructions (from nested block exits) are
		// merged into one PopScope whose count is the sum of the two.  This
		// reduces interpreter dispatch overhead.
		//
		// The OptCount operation in each placeholder accumulates the operand
		// values into register 1.  When the operand is nil it defaults to 1.
		//
		// Pattern:
		//   PopScope  <count1>
		//   PopScope  <count2>
		//
		// Replacement:
		//   PopScope  <count1 + count2>   (read from register 1)
		Description: "Sequential PopScope",
		Pattern: []instruction{
			{
				Operation: PopScope,
				Operand:   placeholder{Name: "count1", Operation: OptCount, Register: 1},
			},
			{
				Operation: PopScope,
				Operand:   placeholder{Name: "count2", Operation: OptCount, Register: 1},
			},
		},
		Replacement: []instruction{
			{
				Operation: PopScope,
				Operand:   placeholder{Name: "count", Operation: optRead, Register: 1},
			},
		},
	},
	{
		// SymbolCreate followed immediately by Store to the same name can be
		// collapsed into a single CreateAndStore instruction.
		//
		// Pattern:
		//   SymbolCreate  <name>
		//   Store         <name>
		//
		// Replacement:
		//   CreateAndStore  <name>
		Description: "Create and store",
		Pattern: []instruction{
			{
				Operation: SymbolCreate,
				Operand:   placeholder{Name: "symbolName"},
			},
			{
				Operation: Store,
				Operand:   placeholder{Name: "symbolName"},
			},
		},
		Replacement: []instruction{
			{
				Operation: CreateAndStore,
				Operand:   placeholder{Name: "symbolName"},
			},
		},
	},

	// ---- StoreIndex / StoreAlways operand folding ---------------------------

	{
		// Push a constant and then StoreIndex (no operand): fold the constant
		// into the StoreIndex operand, removing one stack push.
		//
		// Pattern:
		//   Push        <value>
		//   StoreIndex  (no operand)
		//
		// Replacement:
		//   StoreIndex  <value>
		Description: "Push and Storeindex",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "value"},
			},
			{
				Operation: StoreIndex,
				Operand:   empty{},
			},
		},
		Replacement: []instruction{
			{
				Operation: StoreIndex,
				Operand:   placeholder{Name: "value"},
			},
		},
	},
	{
		// Push a constant and then StoreAlways (with a string variable name):
		// fold the constant into the StoreAlways operand as a two-element slice.
		// The MustBeString constraint ensures we only apply this when the variable
		// name is definitely a string, not some other type.
		//
		// Pattern:
		//   Push        <value>
		//   StoreAlways <name>   (name must be a string)
		//
		// Replacement:
		//   StoreAlways  [<name>, <value>]
		Description: "Constant storeAlways",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "value"},
			},
			{
				Operation: StoreAlways,
				Operand:   placeholder{Name: "name", MustBeString: true},
			},
		},
		Replacement: []instruction{
			{
				Operation: StoreAlways,
				Operand: []any{
					placeholder{Name: "name"},
					placeholder{Name: "value"},
				},
			},
		},
	},

	// ---- Constant folding ---------------------------------------------------
	//
	// The three rules below detect arithmetic on two consecutive Push instructions
	// and replace the pair-plus-opcode with a single Push of the pre-computed
	// result.  The optRunConstantFragment operation executes the original three
	// instructions in an isolated context and captures the resulting value.
	//
	// Example: Push(2) + Push(3) + Add  →  Push(5)
	//
	// Because the pattern uses a bare placeholder{} for the Push operands, it
	// will match any two Pushes — including Pushes of non-numeric types.  If
	// execution of the fragment fails (e.g., type mismatch), executeFragment
	// returns an error and the optimization is aborted for that position.

	{
		// Fold two constant pushes joined by an addition.
		Description: "Constant addition fold",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "v1"},
			},
			{
				Operation: Push,
				Operand:   placeholder{Name: "v2"},
			},
			{
				Operation: Add,
			},
		},
		Replacement: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "sum", Operation: optRunConstantFragment},
			},
		},
	},
	{
		// Fold two constant pushes joined by a subtraction.
		Description: "Constant subtraction fold",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "v1"},
			},
			{
				Operation: Push,
				Operand:   placeholder{Name: "v2"},
			},
			{
				Operation: Sub,
			},
		},
		Replacement: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "difference", Operation: optRunConstantFragment},
			},
		},
	},
	{
		// Fold two constant pushes joined by a multiplication.
		Description: "Constant multiplication fold",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "v1"},
			},
			{
				Operation: Push,
				Operand:   placeholder{Name: "v2"},
			},
			{
				Operation: Mul,
			},
		},
		Replacement: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "product", Operation: optRunConstantFragment},
			},
		},
	},
	{
		// Fold two constant pushes joined by a division.
		Description: "Constant division fold",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "v1"},
			},
			{
				Operation: Push,
				Operand:   placeholder{Name: "v2"},
			},
			{
				Operation: Div,
			},
		},
		Replacement: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "dividend", Operation: optRunConstantFragment},
			},
		},
	},
}
