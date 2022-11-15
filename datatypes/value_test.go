package datatypes

import (
	"testing"
	"time"
)

func TestNewValue(t *testing.T) {
	count := 100000000

	// First test, use interfaces

	var t1a interface{}

	var t1b interface{}

	var t2a Value

	var t2b Value

	start := time.Now()

	for n := 0; n < count; n++ {
		t1a = 101
		t1b = 1

		t1a, t1b = Normalize(t1a, t1b)

		t1c := GetInt(t1a) + GetInt(t1b)
		if t1c != 102 {
			t.Errorf("Failed, value = %v", t1c)
		}
	}

	elapsed1 := time.Since(start)

	start = time.Now()

	for n := 0; n < count; n++ {
		t2a = NewValue(101)
		t2b = NewValue(1)

		t2c := t2a.Add(t2b)
		if t2c.IntValue() != 102 {
			t.Errorf("Failed, value = %v", t2c)
		}
	}

	elapsed2 := time.Since(start)

	// Report the times.
	if elapsed1 != elapsed2 {
		t.Errorf("Different times; interface{}=%v, Value=%v", elapsed1, elapsed2)
	}
}
