package caches

import (
	"testing"
	"time"

	"github.com/tucats/ego/app-cli/ui"
)

func TestAdd(t *testing.T) {
	t.Run("cache test", func(t *testing.T) {
		ui.Active(ui.CacheLogger, true)

		// =======================================================
		// This test is time sensitive. If you change the timing,
		// order, etc. of Add() or Find() calls the test may break.
		// ========================================================

		// Force cache expiration to be 1s. Must set the scanTime
		// value first, as it will be used when cache 1 is created.
		scanTime = "500ms"

		SetExpiration(1, "1s")

		Add(1, "Tom", 101)

		// This find resets the expire for Tom to 1s from now.
		idx, found := Find(1, "Tom")
		if !found {
			t.Errorf("Did not find item for \"Tom\"")

			return
		}

		if id, ok := idx.(int); ok {
			if id != 101 {
				t.Errorf("Did not get correct value for \"Tom\": %d", id)

				return
			}
		} else {
			t.Errorf("Did not get an int back for \"Tom\": %#v", idx)
		}

		time.Sleep(600 * time.Millisecond)
		Add(1, "Mary", 102)

		// Gotta sleep for long enough to pass Tom's expire time, but
		// Not enough for Mary to expire.
		time.Sleep(1100 * time.Millisecond)

		_, found = Find(1, "Tom")
		if found {
			t.Errorf("Should not have found item for \"Tom\"")

			return
		}

		idx, found = Find(1, "Mary")
		if !found {
			t.Errorf("Did not find item for \"Mary\"")

			return
		}

		if id, ok := idx.(int); ok {
			if id != 102 {
				t.Errorf("Did not get correct value for \"Mary\": %d", id)

				return
			}
		} else {
			t.Errorf("Did not get an int back for \"Mary\": %#v", idx)
		}
	})
}
