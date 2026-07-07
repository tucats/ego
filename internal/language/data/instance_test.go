package data

import (
	"reflect"
	"testing"
)

func TestInstanceOfType(t *testing.T) {
	tests := []struct {
		name string
		t    *Type
		want any
	}{
		{
			name: "test with int type",
			t:    IntType,
			want: int(0),
		},
		{
			name: "test with bool type",
			t:    BoolType,
			want: false,
		},
		{
			name: "test with float64 type",
			t:    Float64Type,
			want: float64(0),
		},
		{
			name: "test with string type",
			t:    StringType,
			want: "",
		},
		{
			name: "test with byte type",
			t:    ByteType,
			want: byte(0),
		},
		{
			name: "test with int32 type",
			t:    Int32Type,
			want: int32(0),
		},
		{
			name: "test with interface type",
			t:    InterfaceType,
			want: Wrap(nil),
		},
		{
			name: "test with struct type",
			t:    NewType("testStruct", StructKind, nil),
			want: NewStruct(NewType("testStruct", StructKind, nil)),
		},
		{
			// InstanceOfType for a map type now returns a nil-state map, matching
			// Go's zero value for "var m map[K]V". The *Map wrapper is non-nil (it
			// retains key/value type metadata) but its internal data field is nil.
			// Map literals ("m := map[K]V{}") use $new() → NewMap() separately.
			name: "test with map type",
			t: &Type{
				kind:      MapKind,
				keyType:   StringType,
				valueType: IntType,
			},
			want: NewNilMap(StringType, IntType),
		},
		{
			name: "test with array type",
			t: &Type{
				kind:      ArrayKind,
				valueType: IntType,
			},
			want: NewArray(IntType, 0),
		},
		{
			name: "test with base type",
			t:    IntType,
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InstanceOfType(tt.t); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InstanceOfType(%s) = %#v, want %#v", tt.name, got, tt.want)
			}
		})
	}
}

// TestInstanceOfType_ScalarFastPathMatchesTableScan is the regression test
// for PERFORMANCE.md Finding 2. InstanceOfType's "default" branch (scalar
// kinds: int, float64, string, bool, chan, error, ...) used to find its
// return value by linearly scanning TypeDeclarations and calling
// typeDef.Kind.IsType(t) on each entry; it now dispatches directly on t.kind
// via a switch, for the exact same reason and result, without the scan.
//
// This test is an independent oracle for that change: for every entry in
// TypeDeclarations, it reproduces the OLD linear-scan lookup by hand (not by
// calling InstanceOfType) and asserts that InstanceOfType(entry.Kind) - which
// now uses the NEW switch-based path - returns exactly what the old scan
// would have found. This directly verifies "pure dispatch-strategy change,
// not a behavior change" for every Kind actually present in the table, not
// just the handful spot-checked in TestInstanceOfType above.
func TestInstanceOfType_ScalarFastPathMatchesTableScan(t *testing.T) {
	for _, typeDef := range TypeDeclarations {
		t.Run(typeDef.Kind.String(), func(t *testing.T) {
			// Reproduce the pre-Finding-2 lookup strategy: scan the whole
			// table (starting fresh, exactly as the old code did) and take
			// the first entry whose Kind matches. Kinds handled by
			// InstanceOfType's OTHER switch cases (Array/Map/Pointer/Type/
			// Interface/Struct) are skipped here, because those never
			// reached the linear scan at all - it was, and remains, dead
			// code for them either way.
			switch typeDef.Kind.kind {
			case ArrayKind, MapKind, PointerKind, TypeKind, InterfaceKind, StructKind:
				return
			}

			var want any

			for _, candidate := range TypeDeclarations {
				if candidate.Kind.IsType(typeDef.Kind) {
					want = candidate.Model

					break
				}
			}

			got := InstanceOfType(typeDef.Kind)

			if !reflect.DeepEqual(got, want) {
				t.Errorf("InstanceOfType(%s) = %#v, want %#v (matching the old table-scan result)",
					typeDef.Kind.String(), got, want)
			}
		})
	}
}

// TestInstanceOfType_AllScalarKindsCovered enumerates every Kind handled by
// the new fast-path switch directly (rather than only the ones that happen
// to appear in TypeDeclarations) and confirms each returns a non-nil,
// correctly-typed zero value. This is a belt-and-suspenders check that the
// switch itself - not just its agreement with the old scan - is correct,
// independent of whatever TypeDeclarations happens to contain.
func TestInstanceOfType_AllScalarKindsCovered(t *testing.T) {
	tests := []struct {
		kind int
		want any
	}{
		{BoolKind, false},
		{ByteKind, byte(0)},
		{Int8Kind, int8(0)},
		{Int16Kind, int16(0)},
		{UInt16Kind, uint16(0)},
		{Int32Kind, int32(0)},
		{UInt32Kind, uint32(0)},
		{IntKind, int(0)},
		{UIntKind, uint(0)},
		{Int64Kind, int64(0)},
		{UInt64Kind, uint64(0)},
		{Float32Kind, float32(0)},
		{Float64Kind, float64(0)},
		{StringKind, ""},
		{ChanKind, chanModel},
		{ErrorKind, errorModel},
	}

	for _, tt := range tests {
		t.Run(KindName(tt.kind), func(t *testing.T) {
			got := InstanceOfType(&Type{kind: tt.kind})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InstanceOfType(kind=%s) = %#v, want %#v", KindName(tt.kind), got, tt.want)
			}
		})
	}
}
