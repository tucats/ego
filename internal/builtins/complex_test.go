package builtins

// Tests for the complex(), real(), and imag() builtins (builtins/complex.go).

import (
	"testing"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

func Test_Complex_Float64Args(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	got, err := Complex(s, data.NewList(3.0, 4.0))
	if err != nil {
		t.Fatalf("Complex(3.0, 4.0) unexpected error: %v", err)
	}

	if got != complex128(complex(3, 4)) {
		t.Errorf("Complex(3.0, 4.0) = %v, want (3+4i)", got)
	}
}

func Test_Complex_Float32ArgsProduceComplex64(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	got, err := Complex(s, data.NewList(float32(1.5), float32(2.5)))
	if err != nil {
		t.Fatalf("Complex(float32, float32) unexpected error: %v", err)
	}

	if got != complex64(complex(1.5, 2.5)) {
		t.Errorf("Complex(float32(1.5), float32(2.5)) = %v (%T), want complex64(1.5+2.5i)", got, got)
	}
}

func Test_Complex_MixedIntFloatArgsProduceComplex128(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	got, err := Complex(s, data.NewList(3, 4.0))
	if err != nil {
		t.Fatalf("Complex(3, 4.0) unexpected error: %v", err)
	}

	if got != complex128(complex(3, 4)) {
		t.Errorf("Complex(3, 4.0) = %v (%T), want complex128(3+4i)", got, got)
	}
}

func Test_Complex_InvalidArgError(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	_, err := Complex(s, data.NewList("hello", 4.0))
	if err == nil {
		t.Fatal("Complex(\"hello\", 4.0) should have errored")
	}

	if !errors.Equals(err, errors.ErrArgumentType) {
		t.Errorf("Complex(\"hello\", 4.0) returned wrong error: %v", err)
	}
}

func Test_Real_Complex128(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	got, err := Real(s, data.NewList(complex(3, 4)))
	if err != nil {
		t.Fatalf("Real(3+4i) unexpected error: %v", err)
	}

	if got != float64(3) {
		t.Errorf("Real(3+4i) = %v (%T), want float64(3)", got, got)
	}
}

func Test_Real_Complex64(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	got, err := Real(s, data.NewList(complex64(complex(3, 4))))
	if err != nil {
		t.Fatalf("Real(complex64(3+4i)) unexpected error: %v", err)
	}

	if got != float32(3) {
		t.Errorf("Real(complex64(3+4i)) = %v (%T), want float32(3)", got, got)
	}
}

func Test_Real_InvalidArgError(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	_, err := Real(s, data.NewList(5))
	if err == nil {
		t.Fatal("Real(5) should have errored")
	}

	if !errors.Equals(err, errors.ErrArgumentType) {
		t.Errorf("Real(5) returned wrong error: %v", err)
	}
}

func Test_Imag_Complex128(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	got, err := Imag(s, data.NewList(complex(3, 4)))
	if err != nil {
		t.Fatalf("Imag(3+4i) unexpected error: %v", err)
	}

	if got != float64(4) {
		t.Errorf("Imag(3+4i) = %v (%T), want float64(4)", got, got)
	}
}

func Test_Imag_Complex64(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	got, err := Imag(s, data.NewList(complex64(complex(3, 4))))
	if err != nil {
		t.Fatalf("Imag(complex64(3+4i)) unexpected error: %v", err)
	}

	if got != float32(4) {
		t.Errorf("Imag(complex64(3+4i)) = %v (%T), want float32(4)", got, got)
	}
}

func Test_Imag_InvalidArgError(t *testing.T) {
	s := symbols.NewSymbolTable("test")

	_, err := Imag(s, data.NewList("hello"))
	if err == nil {
		t.Fatal("Imag(\"hello\") should have errored")
	}

	if !errors.Equals(err, errors.ErrArgumentType) {
		t.Errorf("Imag(\"hello\") returned wrong error: %v", err)
	}
}
