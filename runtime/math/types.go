package math

import (
	"math"

	"github.com/tucats/ego/data"
)

var MathPackage = data.NewPackageFromMap("math", map[string]any{
	"Abs":     mathFunc("Abs", math.Abs),
	"Acos":    mathFunc("Acos", math.Acos),
	"Acosh":   mathFunc("Acosh", math.Acosh),
	"Asin":    mathFunc("Asin", math.Asin),
	"Asinh":   mathFunc("Asinh", math.Asinh),
	"Atan":    mathFunc("Atan", math.Atan),
	"Atanh":   mathFunc("Atanh", math.Atanh),
	"Cbrt":    mathFunc("Cbrt", math.Cbrt),
	"Ceil":    mathFunc("Ceil", math.Ceil),
	"Cos":     mathFunc("Cos", math.Cos),
	"Cosh":    mathFunc("Cosh", math.Cosh),
	"Erf":     mathFunc("Erf", math.Erf),
	"Erfc":    mathFunc("Erfc", math.Erfc),
	"Erfcinv": mathFunc("Erfcinv", math.Erfcinv),
	"Erfinv":  mathFunc("Erfinv", math.Erfinv),
	"Exp2":    mathFunc("Exp2", math.Exp2),
	"Expm1":   mathFunc("Expm1", math.Expm1),
	"Fabs":    mathFunc("Fabs", math.Abs),
	"Floor":   mathFunc("Floor", math.Floor),
	"Gamma":   mathFunc("Gamma", math.Gamma),
	"Inf": data.Function{
		Declaration: &data.Declaration{
			Name: "Inf",
			Parameters: []data.Parameter{
				{
					Name: "sign",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.Float64Type},
		},
		Value:    math.Inf,
		IsNative: true,
	},
	"IsInf": data.Function{
		Declaration: &data.Declaration{
			Name: "IsInf",
			Parameters: []data.Parameter{
				{
					Name: "f",
					Type: data.Float64Type,
				},
				{
					Name: "sign",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.BoolType},
		},
		Value:    math.IsInf,
		IsNative: true,
	},
	"IsNan": data.Function{
		Declaration: &data.Declaration{
			Name: "IsNan",
			Parameters: []data.Parameter{
				{
					Name: "f",
					Type: data.Float64Type,
				},
			},
			Returns: []*data.Type{data.BoolType},
		},
		Value:    math.IsNaN,
		IsNative: true,
	},
	"Log": data.Function{
		Declaration: &data.Declaration{
			Name: "Log",
			Parameters: []data.Parameter{
				{
					Name: "f",
					Type: data.Float64Type,
				},
			},
			Returns: []*data.Type{data.Float64Type},
		},
		Value:    math.Log,
		IsNative: true,
	},
	"Max": data.Function{
		Declaration: &data.Declaration{
			Name: "Max",
			Parameters: []data.Parameter{
				{
					Name: "any",
					Type: data.InterfaceType,
				},
			},
			Variadic: true,
			Returns:  []*data.Type{data.InterfaceType},
		},
		Value: maximum,
	},
	"Min": data.Function{
		Declaration: &data.Declaration{
			Name: "Min",
			Parameters: []data.Parameter{
				{
					Name: "any",
					Type: data.InterfaceType,
				},
			},
			Variadic: true,
			Returns:  []*data.Type{data.InterfaceType},
		},
		Value: minimum,
	},
	"Mod": data.Function{
		Declaration: &data.Declaration{
			Name: "Mod",
			Parameters: []data.Parameter{
				{
					Name: "dividend",
					Type: data.Float64Type,
				},
				{
					Name: "divisor",
					Type: data.Float64Type,
				},
			},
			Returns: []*data.Type{data.Float64Type},
		},
		Value:    math.Mod,
		IsNative: true,
	},
	"NaN": data.Function{
		Declaration: &data.Declaration{
			Name:    "NaN",
			Returns: []*data.Type{data.Float64Type},
		},
		Value:    math.NaN,
		IsNative: true,
	},
	"Normalize": data.Function{
		Declaration: &data.Declaration{
			Name: "Normalize",
			Parameters: []data.Parameter{
				{
					Name: "a",
					Type: data.InterfaceType,
				},
				{
					Name: "b",
					Type: data.InterfaceType,
				},
			},
			Variadic: true,
			Returns:  []*data.Type{data.InterfaceType, data.InterfaceType},
		},
		Value: normalize,
	},
	"Random": data.Function{
		Declaration: &data.Declaration{
			Name: "Random",
			Parameters: []data.Parameter{
				{
					Name: "max",
					Type: data.IntType,
				},
			},
			Returns: []*data.Type{data.IntType},
		},
		Value: random,
	},
	"Remainder": data.Function{
		Declaration: &data.Declaration{
			Name: "Remainder",
			Parameters: []data.Parameter{
				{
					Name: "dividend",
					Type: data.Float64Type,
				},
				{
					Name: "divisor",
					Type: data.Float64Type,
				},
			},
			Returns: []*data.Type{data.Float64Type},
		},
		Value:    math.Mod,
		IsNative: true,
	},
	"Round":       mathFunc("Round", math.Round),
	"RoundToEven": mathFunc("RoundToEven", math.RoundToEven),
	"Sin":         mathFunc("Sin", math.Sin),
	"Sinh":        mathFunc("Sinh", math.Sinh),
	"Sqrt":        mathFunc("Sqrt", math.Sqrt),
	"Sum": data.Function{
		Declaration: &data.Declaration{
			Name: "Sum",
			Parameters: []data.Parameter{
				{
					Name: "any",
					Type: data.InterfaceType,
				},
			},
			Variadic: true,
			Returns:  []*data.Type{data.InterfaceType},
		},
		Value: sum,
	},
	"Tan":   mathFunc("Tan", math.Tan),
	"Tanh":  mathFunc("Tanh", math.Tanh),
	"Trunc": mathFunc("Trunc", math.Trunc),
})

func mathFunc(name string, fn func(float64) float64) data.Function {
	return data.Function{
		Declaration: &data.Declaration{
			Name: name,
			Parameters: []data.Parameter{
				{
					Name: "f",
					Type: data.Float64Type,
				},
			},
			Returns: []*data.Type{data.Float64Type},
		},
		Value:    fn,
		IsNative: true,
	}
}
