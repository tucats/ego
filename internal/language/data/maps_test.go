package data

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/errors"
)

func TestNewMapFromMap(t *testing.T) {
	type args struct {
		v any
	}

	tests := []struct {
		name string
		args args
		want *Map
	}{
		{
			name: "int map",
			args: args{v: map[string]int{"tom": 15, "sue": 19}},
			want: &Map{
				keyType:     StringType,
				elementType: IntType,
				immutable:   0,
				data:        map[any]any{"tom": 15, "sue": 19},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewMapFromMap(tt.args.v); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewMapFromMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNewNilMap verifies that NewNilMap produces a non-nil *Map wrapper that
// retains key/value type metadata while leaving the internal data store nil.
func TestNewNilMap(t *testing.T) {
	m := NewNilMap(StringType, IntType)

	if m == nil {
		t.Fatal("NewNilMap returned a nil *Map pointer; want non-nil wrapper")
	}

	if m.data != nil {
		t.Errorf("NewNilMap: data field = %v, want nil (nil-state sentinel)", m.data)
	}

	if !m.keyType.IsType(StringType) {
		t.Errorf("NewNilMap: KeyType = %v, want StringType", m.keyType)
	}

	if !m.elementType.IsType(IntType) {
		t.Errorf("NewNilMap: ElementType = %v, want IntType", m.elementType)
	}

	// TypeString must still work — it uses keyType/elementType, not m.data.
	if got := m.TypeString(); got != "map[string]int" {
		t.Errorf("NewNilMap TypeString() = %q, want %q", got, "map[string]int")
	}
}

// TestNilMapSet verifies that writing to a nil-state map returns ErrNilMapWrite
// and does not modify the map, matching Go's "assignment to entry in nil map"
// panic semantics (converted to a catchable Ego error).
func TestNilMapSet(t *testing.T) {
	m := NewNilMap(StringType, IntType)

	_, err := m.Set("key", 42)
	if err == nil {
		t.Fatal("Set on nil-state map: got nil error, want ErrNilMapWrite")
	}

	if !errors.Equals(err, errors.ErrNilMapWrite) {
		t.Errorf("Set on nil-state map: error = %v, want ErrNilMapWrite", err)
	}

	// The data field must remain nil — no partial mutation should occur.
	if m.data != nil {
		t.Errorf("Set on nil-state map mutated data field; want data == nil")
	}
}

// TestNilMapGet verifies that reading from a nil-state map returns (nil, false,
// nil) with no error, matching Go's nil-map read semantics.
func TestNilMapGet(t *testing.T) {
	m := NewNilMap(StringType, IntType)

	v, found, err := m.Get("key")
	if err != nil {
		t.Fatalf("Get on nil-state map returned unexpected error: %v", err)
	}

	if found {
		t.Errorf("Get on nil-state map: found = true, want false")
	}

	if v != nil {
		t.Errorf("Get on nil-state map: value = %v, want nil", v)
	}
}

// TestNilMapLen verifies that len(nilMap) returns 0, matching Go's nil-map len
// semantics.
func TestNilMapLen(t *testing.T) {
	m := NewNilMap(StringType, IntType)

	if n := m.Len(); n != 0 {
		t.Errorf("Len on nil-state map = %d, want 0", n)
	}
}

// TestNilMapKeys verifies that Keys() on a nil-state map returns nil or an
// empty slice (zero iterations), matching Go's "range over nil map is safe"
// semantics.
func TestNilMapKeys(t *testing.T) {
	m := NewNilMap(StringType, IntType)

	keys := m.Keys()
	if len(keys) != 0 {
		t.Errorf("Keys on nil-state map = %v (len %d), want empty", keys, len(keys))
	}
}

// TestNilMapDelete verifies that Delete() on a nil-state map is a safe no-op
// returning (false, nil), matching Go's spec ("delete on nil map is a no-op").
func TestNilMapDelete(t *testing.T) {
	m := NewNilMap(StringType, IntType)

	found, err := m.Delete("key")
	if err != nil {
		t.Fatalf("Delete on nil-state map returned unexpected error: %v", err)
	}

	if found {
		t.Errorf("Delete on nil-state map: found = true, want false")
	}
}

// TestIsNilForNilStateMap verifies that IsNil correctly identifies a nil-state
// *Map (data field nil, pointer non-nil) as nil from the Ego perspective, so
// that "m == nil" evaluates true in Ego scripts.
func TestIsNilForNilStateMap(t *testing.T) {
	nilState := NewNilMap(StringType, IntType)
	initialized := NewMap(StringType, IntType)

	if !IsNil(nilState) {
		t.Error("IsNil(NewNilMap(...)): got false, want true")
	}

	if IsNil(initialized) {
		t.Error("IsNil(NewMap(...)): got true for initialized map, want false")
	}

	// A literal Go-nil *Map pointer must also be detected.
	var nilPtr *Map
	if !IsNil(nilPtr) {
		t.Error("IsNil((*Map)(nil)): got false, want true")
	}
}

// TestNilMapSetAlwaysVivifies verifies that SetAlways auto-vivifies the
// nil-state map's internal data store so that trusted native Go callers
// (e.g. runtime packages initializing struct-owned map fields) can write
// without triggering an error or a native Go panic.
func TestNilMapSetAlwaysVivifies(t *testing.T) {
	m := NewNilMap(StringType, IntType)

	// SetAlways on a nil-state map must not panic and must make the value
	// retrievable afterward.
	m = m.SetAlways("hello", 99)
	if m == nil {
		t.Fatal("SetAlways returned nil")
	}

	if m.data == nil {
		t.Error("SetAlways did not vivify the data field; m.data is still nil")
	}

	v, found, err := m.Get("hello")
	if err != nil || !found || v != 99 {
		t.Errorf("Get after SetAlways: value=%v found=%v err=%v; want 99, true, nil",
			v, found, err)
	}

	// After vivification, Set() must also succeed (map is no longer nil-state).
	if _, err = m.Set("world", 1); err != nil {
		t.Errorf("Set after SetAlways vivification returned error: %v", err)
	}
}
