package symbols

import (
	"reflect"
	"testing"
)

func Test_SetAlways(t *testing.T) {
	syms := NewRootSymbolTable("test")

	t.Run("set a new symbol", func(t *testing.T) {
		syms.SetAlways("foo", 42)

		attr := syms.symbols["foo"]
		value := syms.GetValue(attr.slot)

		if !reflect.DeepEqual(value, 42) {
			t.Errorf("SetAlways() = %v, want %v", value, 42)
		}
	})

	t.Run("set an exiting symbol", func(t *testing.T) {
		syms.SetAlways("foo", 44)

		attr := syms.symbols["foo"]
		value := syms.GetValue(attr.slot)

		if !reflect.DeepEqual(value, 44) {
			t.Errorf("SetAlways() = %v, want %v", value, 42)
		}
	})

	t.Run("set an readonly symbol", func(t *testing.T) {
		syms.SetAlways("bar", 42)
		_ = syms.SetReadOnly("bar", true)

		syms.SetAlways("bar", 44)
		attr := syms.symbols["foo"]
		value := syms.GetValue(attr.slot)

		if !reflect.DeepEqual(value, 44) {
			t.Errorf("SetAlways() = %v, want %v", value, 42)
		}
	})
}

func Test_Set(t *testing.T) {
	syms := NewRootSymbolTable("test")

	t.Run("set a new symbol", func(t *testing.T) {
		if err := syms.Set("foo", 42); err.Error() != "unknown symbol: foo" {
			t.Errorf("Set(set a new symbol) = %v, want nil", err)
		}
	})

	t.Run("set an exiting symbol", func(t *testing.T) {
		syms.SetAlways("foo", 0)

		if err := syms.Set("foo", 42); err != nil {
			t.Errorf("Set(set an existing symbol) = %v, want nil", err)
		}

		attr := syms.symbols["foo"]
		value := syms.GetValue(attr.slot)

		if !reflect.DeepEqual(value, 42) {
			t.Errorf("Set(set an existing symbol) = %v, want %v", value, 42)
		}
	})

	t.Run("set an readonly symbol", func(t *testing.T) {
		syms.SetAlways("bar", 42)
		_ = syms.SetReadOnly("bar", true)

		if err := syms.Set("bar", 44); err.Error() != "invalid attempt to modify a read-only value: bar" {
			t.Errorf("Set(set an readonly symbol) = %v, want nil", err)
		}

		attr := syms.symbols["foo"]
		value := syms.GetValue(attr.slot)

		if !reflect.DeepEqual(value, 42) {
			t.Errorf("SetAlways() = %v, want %v", value, 42)
		}
	})
}
