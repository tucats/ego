package data

import (
	"testing"
	"time"
)

func TestString(t *testing.T) {
	tests := []struct {
		name string
		arg  any
		want string
	}{
		{name: "nil", arg: nil, want: ""},
		{name: "empty string", arg: "", want: ""},
		{name: "plain string", arg: "hello", want: "hello"},
		{name: "string containing formatting-like text", arg: "%v %d", want: "%v %d"},
		{name: "immutable-wrapped string", arg: Immutable{Value: "wrapped"}, want: "wrapped"},
		{name: "int", arg: 42, want: "42"},
		{name: "bool true", arg: true, want: "true"},
		{name: "bool false", arg: false, want: "false"},
		{name: "float64", arg: 3.5, want: "3.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := String(tt.arg); got != tt.want {
				t.Errorf("String(%#v) = %q, want %q", tt.arg, got, tt.want)
			}
		})
	}
}

func TestString_Time(t *testing.T) {
	when := time.Date(2026, time.July, 7, 13, 4, 5, 0, time.UTC)

	got := String(when)
	want := when.Format(time.RFC822Z)

	if got != want {
		t.Errorf("String(time.Time) = %q, want %q", got, want)
	}
}

func TestComplex128(t *testing.T) {
	tests := []struct {
		name    string
		arg     any
		want    complex128
		wantErr bool
	}{
		{name: "nil", arg: nil, want: 0},
		{name: "int", arg: 5, want: complex(5, 0)},
		{name: "float64", arg: 3.5, want: complex(3.5, 0)},
		{name: "complex64", arg: complex64(complex(1, 2)), want: complex(1, 2)},
		{name: "complex128", arg: complex(3, 4), want: complex(3, 4)},
		{name: "invalid string", arg: "not a complex", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Complex128(tt.arg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Complex128(%#v) expected error, got none", tt.arg)
				}

				return
			}

			if err != nil {
				t.Fatalf("Complex128(%#v) unexpected error: %v", tt.arg, err)
			}

			if got != tt.want {
				t.Errorf("Complex128(%#v) = %v, want %v", tt.arg, got, tt.want)
			}
		})
	}
}

func TestComplex64(t *testing.T) {
	got, err := Complex64(5)
	if err != nil {
		t.Fatalf("Complex64(5) unexpected error: %v", err)
	}

	if got != complex64(complex(5, 0)) {
		t.Errorf("Complex64(5) = %v, want (5+0i)", got)
	}
}
