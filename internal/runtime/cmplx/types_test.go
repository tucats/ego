package cmplx

import "testing"

// Test_CmplxPackage_HasExpectedFunctions is a lightweight structural test
// confirming every documented cmplx function is registered. Behavioral
// coverage (correct results end-to-end through the real dispatch stack)
// lives in tests/cmplx/*.ego, which exercises the actual native-call path
// -- a Go-level call to the underlying math/cmplx function directly would
// not catch dispatch-layer issues like the missing Complex64Kind/
// Complex128Kind cases in convertToNative found while wiring this package up.
func Test_CmplxPackage_HasExpectedFunctions(t *testing.T) {
	want := []string{
		"Abs", "Conj", "Cos", "Exp", "Log", "Log10", "Phase", "Sin", "Sqrt",
		"Tan", "Inf", "NaN", "IsInf", "IsNaN", "Polar", "Pow", "Rect",
	}

	for _, name := range want {
		if _, ok := CmplxPackage.Get(name); !ok {
			t.Errorf("CmplxPackage missing expected function %q", name)
		}
	}
}
