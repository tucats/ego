package bytecode

var Optimizations = []Optimization{
	{
		Description: "Create and store",
		Debug:       true,
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
	{
		Description: "Constant storeAlways",
		Source: []Instruction{
			{
				Operation: Push,
				Operand:   OptimizerToken{Name: "value"},
			},
			{
				Operation: StoreAlways,
				Operand:   OptimizerToken{Name: "name"},
			},
		},
		Replacement: []Instruction{
			{
				Operation: StoreAlways,
				Operand: []interface{}{
					OptimizerToken{Name: "name"},
					OptimizerToken{Name: "value"},
				},
			},
		},
	},
	{
		Description: "Constant addition fold",
		Source: []Instruction{
			{
				Operation: Push,
				Operand:   OptimizerToken{Name: "value1", Operation: OptAdd},
			},
			{
				Operation: Push,
				Operand:   OptimizerToken{Name: "value2", Operation: OptAdd},
			},
			{
				Operation: Add,
			},
		},
		Replacement: []Instruction{
			{
				Operation: Push,
				Operand:   OptimizerToken{Name: "value", Operation: OptAdd},
			},
		},
	},
}
