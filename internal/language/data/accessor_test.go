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
