package bytecode

var Optimizations = []optimization{
	{
		Description: "Load followed by SetThis",
		Pattern: []Instruction{
			{
				Operation: Load,
				Operand:   placeholder{Name: "name"},
			},
			{
				Operation: SetThis,
				Operand:   nil,
			},
		},
		Replacement: []Instruction{
			{
				Operation: LoadThis,
				Operand:   placeholder{Name: "name"},
			},
		},
	},
	{
		Description: "Collapse constant push and createandstore",
		Pattern: []Instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "value"},
			},
			{
				Operation: CreateAndStore,
				Operand:   placeholder{Name: "name"},
			},
		},
		Replacement: []Instruction{
			{
				Operation: CreateAndStore,
				Operand: []interface{}{
					placeholder{Name: "name"},
					placeholder{Name: "value"},
				},
			},
		},
	},
	{
		Description: "Unnecessary stack marker for constant store",
		Pattern: []Instruction{
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
		Replacement: []Instruction{
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
	{
		Description: "Sequential PopScope",
		Pattern: []Instruction{
			{
				Operation: PopScope,
				Operand:   placeholder{Name: "count1", Operation: OptCount, Register: 1},
			},
			{
				Operation: PopScope,
				Operand:   placeholder{Name: "count2", Operation: OptCount, Register: 1},
			},
		},
		Replacement: []Instruction{
			{
				Operation: PopScope,
				Operand:   placeholder{Name: "count", Operation: OptRead, Register: 1},
			},
		},
	}, {
		Description: "Create and store",
		Pattern: []Instruction{
			{
				Operation: SymbolCreate,
				Operand:   placeholder{Name: "symbolName"},
			},
			{
				Operation: Store,
				Operand:   placeholder{Name: "symbolName"},
			},
		},
		Replacement: []Instruction{
			{
				Operation: CreateAndStore,
				Operand:   placeholder{Name: "symbolName"},
			},
		},
	},
	{
		Description: "Push and Storeindex",
		Pattern: []Instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "value"},
			},
			{
				Operation: StoreIndex,
			},
		},
		Replacement: []Instruction{
			{
				Operation: StoreIndex,
				Operand:   placeholder{Name: "value"},
			},
		},
	},
	{
		Description: "Constant storeAlways",
		Pattern: []Instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "value"},
			},
			{
				Operation: StoreAlways,
				Operand:   placeholder{Name: "name"},
			},
		},
		Replacement: []Instruction{
			{
				Operation: StoreAlways,
				Operand: []interface{}{
					placeholder{Name: "name"},
					placeholder{Name: "value"},
				},
			},
		},
	},
	{
		Description: "Constant addition fold",
		Pattern: []Instruction{
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
		Replacement: []Instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "sum", Operation: OptRunConstantFragment},
			},
		},
	},
	{
		Description: "Constant subtraction fold",
		Pattern: []Instruction{
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
		Replacement: []Instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "difference", Operation: OptRunConstantFragment},
			},
		},
	},
	{
		Description: "Constant multiplication fold",
		Pattern: []Instruction{
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
		Replacement: []Instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "product", Operation: OptRunConstantFragment},
			},
		},
	},
}
