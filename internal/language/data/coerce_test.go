package data

import (
	"testing"

	"github.com/tucats/ego/internal/errors"
)

func TestCoerce(t *testing.T) {
	type args struct {
		v     any
		model any
	}

	tests := []struct {
		name string
		args args
		want any
	}{
		{
			name: "test with float32 int64 model",
			args: args{
				v:     float32(100.5),
				model: int64(0),
			},
			want: int64(100),
		},
		{
			name: "test with float32 int32 model",
			args: args{
				v:     float32(100.5),
				model: int32(0),
			},
			want: int32(100),
		},
		{
			name: "test with nil byte model",
			args: args{
				v:     1,
				model: byte(0),
			},
			want: byte(1),
		},
		{
			name: "test with bool byte model",
			args: args{
				v:     true,
				model: byte(0),
			},
			want: byte(1),
		},
		{
			name: "test with byte/byte model",
			args: args{
				v:     byte(1),
				model: byte(0),
			},
			want: byte(1),
		},
		{
			name: "test with int byte model",
			args: args{
				v:     100,
				model: byte(0),
			},
			want: byte(100),
		},
		{
			name: "test with int32 byte model",
			args: args{
				v:     int32(100),
				model: byte(0),
			},
			want: byte(100),
		},
		{
			name: "test with int64 byte model",
			args: args{
				v:     int64(100),
				model: byte(0),
			},
			want: byte(100),
		},
		{
			name: "test with float32 byte model",
			args: args{
				v:     float32(100.5),
				model: byte(0),
			},
			want: byte(100),
		},
		{
			name: "test with float64 byte model",
			args: args{
				v:     float64(100.5),
				model: byte(0),
			},
			want: byte(100),
		},
		{
			name: "test with string byte model",
			args: args{
				v:     "100",
				model: byte(0),
			},
			want: byte(100),
		},
		{
			name: "test with nil int32 model",
			args: args{
				v:     1,
				model: int32(0),
			},
			want: int32(1),
		},
		{
			name: "test with bool int32 model",
			args: args{
				v:     true,
				model: int32(0),
			},
			want: int32(1),
		},
		{
			name: "test with byte int32 model",
			args: args{
				v:     byte(1),
				model: int32(0),
			},
			want: int32(1),
		},
		{
			name: "test with int int32 model",
			args: args{
				v:     100,
				model: int32(0),
			},
			want: int32(100),
		},
		{
			name: "test with int32/int32 model",
			args: args{
				v:     int32(100),
				model: int32(0),
			},
			want: int32(100),
		},
		{
			name: "test with int64 int32 model",
			args: args{
				v:     int64(100),
				model: int32(0),
			},
			want: int32(100),
		},
		{
			name: "test with float64 int32 model",
			args: args{
				v:     float64(100.5),
				model: int32(0),
			},
			want: int32(100),
		},
		{
			name: "test with string int32 model",
			args: args{
				v:     "100",
				model: int32(0),
			},
			want: int32(100),
		},
		{
			name: "test with nil int64 model",
			args: args{
				v:     1,
				model: int64(0),
			},
			want: int64(1),
		},
		{
			name: "test with bool int64 model",
			args: args{
				v:     true,
				model: int64(0),
			},
			want: int64(1),
		},
		{
			name: "test with byte int64 model",
			args: args{
				v:     byte(1),
				model: int64(0),
			},
			want: int64(1),
		},
		{
			name: "test with int int64 model",
			args: args{
				v:     100,
				model: int64(0),
			},
			want: int64(100),
		},
		{
			name: "test with int32 int64 model",
			args: args{
				v:     int32(100),
				model: int64(0),
			},
			want: int64(100),
		},
		{
			name: "test with int64/int64 model",
			args: args{
				v:     int64(100),
				model: int64(0),
			},
			want: int64(100),
		},
		{
			name: "test with float64 int64 model",
			args: args{
				v:     float64(100.5),
				model: int64(0),
			},
			want: int64(100),
		},
		{
			name: "test with string int64 model",
			args: args{
				v:     "100",
				model: int64(0),
			},
			want: int64(100),
		},
		{
			name: "test with nil int model",
			args: args{
				v:     1,
				model: int(0),
			},
			want: 1,
		},
		{
			name: "test with bool int model",
			args: args{
				v:     true,
				model: int(0),
			},
			want: 1,
		},
		{
			name: "test with byte int model",
			args: args{
				v:     byte(1),
				model: int(0),
			},
			want: 1,
		},
		{
			name: "test with int/int model",
			args: args{
				v:     100,
				model: int(0),
			},
			want: 100,
		},
		{
			name: "test with int32 int model",
			args: args{
				v:     int32(100),
				model: int(0),
			},
			want: 100,
		},
		{
			name: "test with int64 int model",
			args: args{
				v:     int64(100),
				model: int(0),
			},
			want: 100,
		},
		{
			name: "test with float32 int model",
			args: args{
				v:     float32(100.5),
				model: int(0),
			},
			want: 100,
		},
		{
			name: "test with float64 int model",
			args: args{
				v:     float64(100.5),
				model: int(0),
			},
			want: 100,
		},
		{
			name: "test with string int model",
			args: args{
				v:     "100",
				model: int(0),
			},
			want: 100,
		},
		{
			name: "test with nil float32 model",
			args: args{
				v:     1,
				model: float32(0),
			},
			want: float32(1),
		},
		{
			name: "test with bool float32 model",
			args: args{
				v:     true,
				model: float32(0),
			},
			want: float32(1),
		},
		{
			name: "test with byte float32 model",
			args: args{
				v:     byte(1),
				model: float32(0),
			},
			want: float32(1),
		},
		{
			name: "test with int32 float32 model",
			args: args{
				v:     int32(100),
				model: float32(0),
			},
			want: float32(100),
		},
		{
			name: "test with int float32 model",
			args: args{
				v:     100,
				model: float32(0),
			},
			want: float32(100),
		},
		{
			name: "test with int64 float32 model",
			args: args{
				v:     int64(100),
				model: float32(0),
			},
			want: float32(100),
		},
		{
			name: "test with float32/float32 model",
			args: args{
				v:     float32(100.5),
				model: float32(0),
			},
			want: float32(100.5),
		},
		{
			name: "test with float64 float32 model",
			args: args{
				v:     float64(100.5),
				model: float32(0),
			},
			want: float32(100.5),
		},
		{
			name: "test with string float32 model",
			args: args{
				v:     "100.5",
				model: float32(0),
			},
			want: float32(100.5),
		},
		{
			name: "test with nil float64 model",
			args: args{
				v:     1,
				model: float64(0),
			},
			want: float64(1),
		},
		{
			name: "test with bool float64 model",
			args: args{
				v:     true,
				model: float64(0),
			},
			want: float64(1),
		},
		{
			name: "test with byte float64 model",
			args: args{
				v:     byte(1),
				model: float64(0),
			},
			want: float64(1),
		},
		{
			name: "test with int32 float64 model",
			args: args{
				v:     int32(100),
				model: float64(0),
			},
			want: float64(100),
		},
		{
			name: "test with int float64 model",
			args: args{
				v:     100,
				model: float64(0),
			},
			want: float64(100),
		},
		{
			name: "test with int64 float64 model",
			args: args{
				v:     int64(100),
				model: float64(0),
			},
			want: float64(100),
		},
		{
			name: "test with float32 float64 model",
			args: args{
				v:     float32(100.5),
				model: float64(0),
			},
			want: float64(100.5),
		},
		{
			name: "test with float64/float64 model",
			args: args{
				v:     float64(100.5),
				model: float64(0),
			},
			want: float64(100.5),
		},
		{
			name: "test with string float64 model",
			args: args{
				v:     "100.5",
				model: float64(0),
			},
			want: float64(100.5),
		},
		{
			name: "test with bool string model",
			args: args{
				v:     true,
				model: "",
			},
			want: "true",
		},
		{
			name: "test with byte string model",
			args: args{
				v:     byte(1),
				model: "",
			},
			want: "1",
		},
		{
			name: "test with int string model",
			args: args{
				v:     100,
				model: "",
			},
			want: "100",
		},
		{
			name: "test with int32 string model",
			args: args{
				v:     int32(100),
				model: "",
			},
			want: "100",
		},
		{
			name: "test with int64 string model",
			args: args{
				v:     int64(100),
				model: "",
			},
			want: "100",
		},
		{
			name: "test with float32 string model",
			args: args{
				v:     float32(100.5),
				model: "",
			},
			want: "100.5",
		},
		{
			name: "test with float64 string model",
			args: args{
				v:     float64(100.5),
				model: "",
			},
			want: "100.5",
		},
		{
			name: "test with string/string model",
			args: args{
				v:     "hello",
				model: "",
			},
			want: "hello",
		},
		{
			name: "test with nil bool model",
			args: args{
				v:     nil,
				model: false,
			},
			want: false,
		},
		{
			name: "test with bool/bool model",
			args: args{
				v:     true,
				model: false,
			},
			want: true,
		},
		{
			name: "test with byte bool model",
			args: args{
				v:     byte(1),
				model: false,
			},
			want: true,
		},
		{
			name: "test with int32 bool model",
			args: args{
				v:     int32(100),
				model: false,
			},
			want: true,
		},
		{
			name: "test with int bool model",
			args: args{
				v:     100,
				model: false,
			},
			want: true,
		},
		{
			name: "test with int64 bool model",
			args: args{
				v:     int64(100),
				model: false,
			},
			want: true,
		},
		{
			name: "test with float32 bool model",
			args: args{
				v:     float32(100.5),
				model: false,
			},
			want: true,
		},
		{
			name: "test with float64 bool model",
			args: args{
				v:     float64(100.5),
				model: false,
			},
			want: true,
		},
		{
			name: "test with string bool model",
			args: args{
				v:     "true",
				model: false,
			},
			want: true,
		},
		{
			name: "test with empty string bool model",
			args: args{
				v:     "",
				model: false,
			},
			want: false,
		},
		{
			// Regression test: coercing a nil value to the *errors.Error
			// model (the "error" type) must stay nil, matching Go's zero
			// value for error. Before the fix, this produced a non-nil
			// error whose message was the literal string "<nil>".
			name: "test with nil error model stays nil",
			args: args{
				v:     nil,
				model: &errors.Error{},
			},
			want: nil,
		},
		{
			// Real numbers coerce implicitly to complex128 (real part =
			// value, imag = 0) -- an Ego extension beyond Go, which requires
			// an explicit conversion.
			name: "test with int complex128 model",
			args: args{
				v:     5,
				model: complex128(0),
			},
			want: complex128(5),
		},
		{
			name: "test with float64 complex128 model",
			args: args{
				v:     3.5,
				model: complex128(0),
			},
			want: complex(3.5, 0),
		},
		{
			name: "test with int complex64 model",
			args: args{
				v:     5,
				model: complex64(0),
			},
			want: complex64(5),
		},
		{
			name: "test with complex64 complex128 model (widening)",
			args: args{
				v:     complex64(complex(1, 2)),
				model: complex128(0),
			},
			want: complex(1, 2),
		},
		{
			name: "test with complex128 complex64 model (narrowing)",
			args: args{
				v:     complex(1, 2),
				model: complex64(0),
			},
			want: complex64(complex(1, 2)),
		},
		{
			name: "test with string complex128 model",
			args: args{
				v:     "3+4i",
				model: complex128(0),
			},
			want: complex(3, 4),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := Coerce(tt.args.v, tt.args.model); got != tt.want {
				t.Errorf("Coerce(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// TestCoerceComplexToRealRejected confirms that complex -> real (int/float)
// coercion is never implicit, in either direction of the two width pairs, and
// across every narrower integer target too -- matching Go's own strictness in
// this direction while real -> complex remains implicit (see TestCoerce above).
func TestCoerceComplexToRealRejected(t *testing.T) {
	models := []any{
		int(0), int8(0), int16(0), int32(0), int64(0),
		uint(0), uint16(0), uint32(0), uint64(0), byte(0),
		float32(0), float64(0), false,
	}

	sources := []any{complex64(complex(3, 4)), complex(3, 4)}

	for _, model := range models {
		for _, source := range sources {
			_, err := Coerce(source, model)
			if err == nil {
				t.Errorf("Coerce(%T, %T) should have errored, did not", source, model)
			}

			if !errors.Equals(err, errors.ErrInvalidComplexValue) {
				t.Errorf("Coerce(%T, %T) returned wrong error: %v", source, model, err)
			}
		}
	}
}

// TestCoerceLosslessComplex is a regression test for the highest-risk item
// identified while planning complex-number support: CoerceLossless is the
// shared implementation backing every strict-mode boundary (assignment, call
// arguments, struct fields, return values). It round-trips through Float64(),
// which now errors for a complex value -- confirm that error is absorbed
// (skipping the lossy-precision check) rather than propagated, so a
// real -> complex coercion still succeeds losslessly in strict mode.
func TestCoerceLosslessComplex(t *testing.T) {
	got, err := CoerceLossless(5, complex128(0))
	if err != nil {
		t.Fatalf("CoerceLossless(5, complex128(0)) returned error: %v", err)
	}

	if got != complex128(5) {
		t.Errorf("CoerceLossless(5, complex128(0)) = %v, want (5+0i)", got)
	}

	got, err = CoerceLossless(3.5, complex64(0))
	if err != nil {
		t.Fatalf("CoerceLossless(3.5, complex64(0)) returned error: %v", err)
	}

	if got != complex64(complex(3.5, 0)) {
		t.Errorf("CoerceLossless(3.5, complex64(0)) = %v, want (3.5+0i)", got)
	}
}

// TestIsNumericKindComplex confirms Complex64Kind/Complex128Kind are
// recognized by isNumericKind's package-private equivalent via the exported
// IsNumeric function (isNumericKind itself lives in the bytecode package and
// is exercised indirectly by TestCoerceLosslessComplex's strict-mode path).
func TestIsNumericComplex(t *testing.T) {
	if !IsNumeric(complex64(0)) {
		t.Error("IsNumeric(complex64(0)) should be true")
	}

	if !IsNumeric(complex128(0)) {
		t.Error("IsNumeric(complex128(0)) should be true")
	}

	if !IsNumeric(Complex64Type) {
		t.Error("IsNumeric(Complex64Type) should be true")
	}

	if !IsNumeric(Complex128Type) {
		t.Error("IsNumeric(Complex128Type) should be true")
	}
}

// TestNormalizeComplex confirms Normalize correctly promotes a non-complex
// operand to complex128 when paired with a complex128 operand, both when the
// non-complex side is a constant (the common case for expressions like
// "c + 5", where c is a complex128 variable) and when it is not.
func TestNormalizeComplex(t *testing.T) {
	v1, v2, err := Normalize(complex128(5), false, 3, true, false)
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	if v1 != complex128(5) || v2 != complex128(3) {
		t.Errorf("Normalize(complex128(5), 3) = (%v, %v), want (5+0i, 3+0i)", v1, v2)
	}

	// Strict mode routes the constant-adaptation branch through
	// CoerceLossless -- confirm it still succeeds (this is the same
	// interaction TestCoerceLosslessComplex exercises directly).
	v1, v2, err = Normalize(complex128(5), false, 3, true, true)
	if err != nil {
		t.Fatalf("Normalize (strict) returned error: %v", err)
	}

	if v1 != complex128(5) || v2 != complex128(3) {
		t.Errorf("Normalize strict(complex128(5), 3) = (%v, %v), want (5+0i, 3+0i)", v1, v2)
	}
}
