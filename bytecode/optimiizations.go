package bytecode

var Optimizations = []Optimization{
	{
		Description: "Sequential PopScope",
		Source: []Instruction{
			{
				Operation: PopScope,
				Operand:   OptimizerToken{Name: "count1", Operation: OptCount, Register: 1},
			},
			{
				Operation: PopScope,
				Operand:   OptimizerToken{Name: "count2", Operation: OptCount, Register: 1},
			},
		},
		Replacement: []Instruction{
			{
				Operation: PopScope,
				Operand:   OptimizerToken{Name: "count", Operation: OptRead, Register: 1},
			},
		},
	}, {
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
	{
		Description: "Push and Storeindex",
		Source: []Instruction{
			{
				Operation: Push,
				Operand:   OptimizerToken{Name: "value"},
			},
			{
				Operation: StoreIndex,
			},
		},
		Replacement: []Instruction{
			{
				Operation: StoreIndex,
				Operand:   OptimizerToken{Name: "value"},
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
				Operand:   OptimizerToken{Name: "v1"},
			},
			{
				Operation: Push,
				Operand:   OptimizerToken{Name: "v2"},
			},
			{
				Operation: Add,
			},
		},
		Replacement: []Instruction{
			{
				Operation: Push,
				Operand:   OptimizerToken{Name: "sum", Operation: OptRunConstantFragment},
			},
		},
	},
	{
		Description: "Constant subtraction fold",
		Source: []Instruction{
			{
				Operation: Push,
				Operand:   OptimizerToken{Name: "v1"},
			},
			{
				Operation: Push,
				Operand:   OptimizerToken{Name: "v2"},
			},
			{
				Operation: Sub,
			},
		},
		Replacement: []Instruction{
			{
				Operation: Push,
				Operand:   OptimizerToken{Name: "difference", Operation: OptRunConstantFragment},
			},
		},
	},
	{
		Description: "Constant multiplication fold",
		Source: []Instruction{
			{
				Operation: Push,
				Operand:   OptimizerToken{Name: "v1"},
			},
			{
				Operation: Push,
				Operand:   OptimizerToken{Name: "v2"},
			},
			{
				Operation: Mul,
			},
		},
		Replacement: []Instruction{
			{
				Operation: Push,
				Operand:   OptimizerToken{Name: "product", Operation: OptRunConstantFragment},
			},
		},
	},
}
