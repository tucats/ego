package data

import (
	"testing"

	"github.com/tucats/ego/internal/errors"
)

// Regression tests for the INDEX-7, INDEX-8, and INDEX-9 fixes. All three
// concern bounds checks in *Array that consulted len(a.data) even for a []byte
// array, whose elements live in a.bytes with a.data left empty.

// INDEX-7: a reversed range passed the old guard (which never compared the two
// ends to each other) and panicked on the slice expression.
func TestGetSliceReversedRange_INDEX7(t *testing.T) {
	array := NewArrayFromInterfaces(IntType, 10, 20, 30, 40, 50)

	slice, err := array.GetSlice(4, 1)
	if err == nil {
		t.Errorf("GetSlice(4, 1) = %v, want ErrArrayBounds", slice)
	} else if !errors.Equal(err, errors.ErrArrayBounds) {
		t.Errorf("GetSlice(4, 1) error = %v, want ErrArrayBounds", err)
	}
}

// INDEX-7: for a byte array the old guard compared both ends against
// len(a.data), which is zero, so every non-empty slice was rejected.
func TestGetSliceOfByteArray_INDEX7(t *testing.T) {
	array := NewArrayFromBytes(1, 2, 3, 4, 5)

	slice, err := array.GetSlice(1, 4)
	if err != nil {
		t.Fatalf("GetSlice(1, 4) unexpected error %v", err)
	}

	if len(slice) != 3 {
		t.Fatalf("GetSlice(1, 4) returned %d elements, want 3", len(slice))
	}

	for index, want := range []byte{2, 3, 4} {
		if got, _ := Byte(slice[index]); got != want {
			t.Errorf("GetSlice(1, 4)[%d] = %v, want %v", index, got, want)
		}
	}

	// A range past the end of the byte array must still be rejected.
	if _, err := array.GetSlice(0, 6); !errors.Equal(err, errors.ErrArrayBounds) {
		t.Errorf("GetSlice(0, 6) error = %v, want ErrArrayBounds", err)
	}
}

// INDEX-8: SetAlways compared the index against len(a.data), so on a byte array
// it rejected every index -- including 0 -- and silently wrote nothing.
func TestSetAlwaysOnByteArray_INDEX8(t *testing.T) {
	array := NewArrayFromBytes(1, 2, 3)

	array.SetAlways(1, byte(99))

	value, err := array.Get(1)
	if err != nil {
		t.Fatalf("Get(1) unexpected error %v", err)
	}

	if got, _ := Byte(value); got != 99 {
		t.Errorf("after SetAlways(1, 99), Get(1) = %v, want 99", got)
	}

	// An out-of-range index remains a no-op rather than a panic.
	array.SetAlways(3, byte(1))
	array.SetAlways(-1, byte(1))

	if array.Len() != 3 {
		t.Errorf("array length = %d, want 3", array.Len())
	}
}

// INDEX-9: growing a byte array padded by size-len(a.data) rather than
// size-len(a.bytes), leaving the array at len(a.bytes)+size elements.
func TestSetSizeGrowsByteArray_INDEX9(t *testing.T) {
	array := NewArrayFromBytes(1, 2, 3)

	if got := array.SetSize(6).Len(); got != 6 {
		t.Errorf("SetSize(6) produced length %d, want 6", got)
	}

	// The original contents must survive the grow.
	for index, want := range []byte{1, 2, 3, 0, 0, 0} {
		value, err := array.Get(index)
		if err != nil {
			t.Fatalf("Get(%d) unexpected error %v", index, err)
		}

		if got, _ := Byte(value); got != want {
			t.Errorf("after SetSize(6), Get(%d) = %v, want %v", index, got, want)
		}
	}

	// Shrinking still works, and was never broken.
	if got := array.SetSize(2).Len(); got != 2 {
		t.Errorf("SetSize(2) produced length %d, want 2", got)
	}
}
