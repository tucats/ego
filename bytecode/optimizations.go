package bytecode

var optimizations = []optimization{
	{
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
	{
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

	{
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
	{
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
	{
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
