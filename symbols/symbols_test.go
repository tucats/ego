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
		value := syms.getValue(attr.slot)

		if !reflect.DeepEqual(value, 42) {
			t.Errorf("SetAlways() = %v, want %v", value, 42)
		}
	})

	t.Run("set an exiting symbol", func(t *testing.T) {
		syms.SetAlways("foo", 44)

		attr := syms.symbols["foo"]
		value := syms.getValue(attr.slot)

		if !reflect.DeepEqual(value, 44) {
			t.Errorf("SetAlways() = %v, want %v", value, 42)
		}
	})

	t.Run("set an readonly symbol", func(t *testing.T) {
		syms.SetAlways("bar", 42)
		_ = syms.SetReadOnly("bar", true)

		syms.SetAlways("bar", 44)
		attr := syms.symbols["foo"]
		value := syms.getValue(attr.slot)

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
		value := syms.getValue(attr.slot)

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
		value := syms.getValue(attr.slot)

		if !reflect.DeepEqual(value, 42) {
			t.Errorf("SetAlways() = %v, want %v", value, 42)
		}
	})
}
func Test_Get(t *testing.T) {
	syms := NewRootSymbolTable("test")

	t.Run("get an existing symbol", func(t *testing.T) {
		syms.SetAlways("foo", 42)

		value, found := syms.Get("foo")

		if !found {
			t.Errorf("Get() = %v, want %v", found, true)
		}

		if !reflect.DeepEqual(value, 42) {
			t.Errorf("Get() = %v, want %v", value, 42)
		}
	})

	t.Run("get a non-existing symbol", func(t *testing.T) {
		value, found := syms.Get("bar")

		if found {
			t.Errorf("Get() = %v, want %v", found, false)
		}

		if value != nil {
			t.Errorf("Get() = %v, want %v", value, nil)
		}
	})

	t.Run("get symbol from parent table", func(t *testing.T) {
		parentSyms := NewRootSymbolTable("parent")
		parentSyms.SetAlways("foo", 42)

		syms.SetParent(parentSyms)

		value, found := syms.Get("foo")

		if !found {
			t.Errorf("Get() = %v, want %v", found, true)
		}

		if !reflect.DeepEqual(value, 42) {
			t.Errorf("Get() = %v, want %v", value, 42)
		}
	})

	t.Run("get symbol from parent table with loop", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Get() did not panic")
			}
		}()

		// Create a parent table, that has a parent loop. The SetParent
		// should panic.
		parentSyms := NewRootSymbolTable("parent")
		parentSyms.SetParent(parentSyms)

		// Attach our testing table to the parent table loop just created.
		syms.SetParent(parentSyms)

		// This should panic
		syms.Get("fooze")
	})
}
func Test_GetLocal(t *testing.T) {
	syms := NewRootSymbolTable("test")

	t.Run("get an existing symbol", func(t *testing.T) {
		syms.SetAlways("foo", 42)

		value, found := syms.GetLocal("foo")

		if !found {
			t.Errorf("GetLocal() = %v, want %v", found, true)
		}

		if !reflect.DeepEqual(value, 42) {
			t.Errorf("GetLocal() = %v, want %v", value, 42)
		}
	})

	t.Run("get a non-existing symbol", func(t *testing.T) {
		// The symbol exists, but not in the local table. This verifies that the
		// search for a symbol stops at the provided local table.
		parent := NewRootSymbolTable("parent")
		parent.SetAlways("bar", "baz")
		syms.SetParent(parent)

		value, found := syms.GetLocal("bar")

		if found {
			t.Errorf("GetLocal() = %v, want %v", found, false)
		}

		if value != nil {
			t.Errorf("GetLocal() = %v, want %v", value, nil)
		}
	})

	t.Run("get symbol with shared symbol table", func(t *testing.T) {
		syms.SetAlways("foo", 42)
		syms.shared = true

		value, found := syms.GetLocal("foo")

		if !found {
			t.Errorf("GetLocal() = %v, want %v", found, true)
		}

		if !reflect.DeepEqual(value, 42) {
			t.Errorf("GetLocal() = %v, want %v", value, 42)
		}
	})

	t.Run("get symbol with shared symbol table and no symbol found", func(t *testing.T) {
		// The symbol exists, but not in the local table. This verifies that the
		// search for a symbol stops at the provided local table.
		parent := NewRootSymbolTable("parent")
		parent.SetAlways("bar", "baz")
		syms.SetParent(parent)
		syms.shared = true

		value, found := syms.GetLocal("bar")

		if found {
			t.Errorf("GetLocal() = %v, want %v", found, false)
		}

		if value != nil {
			t.Errorf("GetLocal() = %v, want %v", value, nil)
		}
	})
}
