package bytecode

var optimizations = []optimization{
	{
		Description: "Load followed by SetThis",
		Pattern: []instruction{
			{
				Operation: Load,
				Operand:   placeholder{Name: "name"},
			},
			{
				Operation: SetThis,
				Operand:   nil,
			},
		},
		Replacement: []instruction{
			{
				Operation: LoadThis,
				Operand:   placeholder{Name: "name"},
			},
		},
	},
	{
		Description: "Collapse constant push and createandstore",
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
				Operand: []interface{}{
					placeholder{Name: "name"},
					placeholder{Name: "value"},
				},
			},
		},
	},
	{
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
	{
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
	}, {
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
	{
		Description: "Push and Storeindex",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "value"},
			},
			{
				Operation: StoreIndex,
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
		Description: "Constant storeAlways",
		Pattern: []instruction{
			{
				Operation: Push,
				Operand:   placeholder{Name: "value"},
			},
			{
				Operation: StoreAlways,
				Operand:   placeholder{Name: "name"},
			},
		},
		Replacement: []instruction{
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
}
