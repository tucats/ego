// Package cmplx provides the Ego "cmplx" package, mirroring Go's standard
// library math/cmplx. Unlike math (which has separate float32/float64
// callers via Ego's own dynamic dispatch), cmplx functions operate on
// complex128 only, matching real Go's own math/cmplx -- there is no
// complex64 variant in the Go standard library either. A complex64 value
// must be explicitly cast with complex128(x) before use with this package.
package cmplx

import (
	"math/cmplx"

	"github.com/tucats/ego/internal/language/data"
)

var CmplxPackage = data.NewPackageFromMap("cmplx", map[string]any{
	"Abs":   cmplxToFloatFunc("Abs", cmplx.Abs),
	"Conj":  cmplxFunc("Conj", cmplx.Conj),
	"Cos":   cmplxFunc("Cos", cmplx.Cos),
	"Exp":   cmplxFunc("Exp", cmplx.Exp),
	"Log":   cmplxFunc("Log", cmplx.Log),
	"Log10": cmplxFunc("Log10", cmplx.Log10),
	"Phase": cmplxToFloatFunc("Phase", cmplx.Phase),
	"Sin":   cmplxFunc("Sin", cmplx.Sin),
	"Sqrt":  cmplxFunc("Sqrt", cmplx.Sqrt),
	"Tan":   cmplxFunc("Tan", cmplx.Tan),
	"Inf": data.Function{
		Declaration: &data.Declaration{
			Name:    "Inf",
			Returns: []*data.Type{data.Complex128Type},
		},
		Value:    cmplx.Inf,
		IsNative: true,
	},
	"NaN": data.Function{
		Declaration: &data.Declaration{
			Name:    "NaN",
			Returns: []*data.Type{data.Complex128Type},
		},
		Value:    cmplx.NaN,
		IsNative: true,
	},
	"IsInf": data.Function{
		Declaration: &data.Declaration{
			Name: "IsInf",
			Parameters: []data.Parameter{
				{
					Name: "x",
					Type: data.Complex128Type,
				},
			},
			Returns: []*data.Type{data.BoolType},
		},
		Value:    cmplx.IsInf,
		IsNative: true,
	},
	"IsNaN": data.Function{
		Declaration: &data.Declaration{
			Name: "IsNaN",
			Parameters: []data.Parameter{
				{
					Name: "x",
					Type: data.Complex128Type,
				},
			},
			Returns: []*data.Type{data.BoolType},
		},
		Value:    cmplx.IsNaN,
		IsNative: true,
	},
	"Polar": data.Function{
		Declaration: &data.Declaration{
			Name: "Polar",
			Parameters: []data.Parameter{
				{
					Name: "x",
					Type: data.Complex128Type,
				},
			},
			Returns: []*data.Type{data.Float64Type, data.Float64Type},
		},
		Value:    cmplx.Polar,
		IsNative: true,
	},
	"Pow": data.Function{
		Declaration: &data.Declaration{
			Name: "Pow",
			Parameters: []data.Parameter{
				{
					Name: "x",
					Type: data.Complex128Type,
				},
				{
					Name: "y",
					Type: data.Complex128Type,
				},
			},
			Returns: []*data.Type{data.Complex128Type},
		},
		Value:    cmplx.Pow,
		IsNative: true,
	},
	"Rect": data.Function{
		Declaration: &data.Declaration{
			Name: "Rect",
			Parameters: []data.Parameter{
				{
					Name: "r",
					Type: data.Float64Type,
				},
				{
					Name: "theta",
					Type: data.Float64Type,
				},
			},
			Returns: []*data.Type{data.Complex128Type},
		},
		Value:    cmplx.Rect,
		IsNative: true,
	},
})

// cmplxFunc builds a native-passthrough data.Function for the common
// complex128 -> complex128 signature shape (Conj, Sqrt, Exp, Log, ...),
// mirroring the internal/runtime/math package's mathFunc helper.
func cmplxFunc(name string, fn func(complex128) complex128) data.Function {
	return data.Function{
		Declaration: &data.Declaration{
			Name: name,
			Parameters: []data.Parameter{
				{
					Name: "x",
					Type: data.Complex128Type,
				},
			},
			Returns: []*data.Type{data.Complex128Type},
		},
		Value:    fn,
		IsNative: true,
	}
}

// cmplxToFloatFunc builds a native-passthrough data.Function for the
// complex128 -> float64 signature shape (Abs, Phase).
func cmplxToFloatFunc(name string, fn func(complex128) float64) data.Function {
	return data.Function{
		Declaration: &data.Declaration{
			Name: name,
			Parameters: []data.Parameter{
				{
					Name: "x",
					Type: data.Complex128Type,
				},
			},
			Returns: []*data.Type{data.Float64Type},
		},
		Value:    fn,
		IsNative: true,
	}
}
