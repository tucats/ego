package caches

import (
	"testing"
	"time"
)

func TestAdd(t *testing.T) {
	t.Run("cache test", func(t *testing.T) {
		scanTime = "1s"
		expireTime = "1s"

		Add(1, "Tom", 101)
		time.Sleep(500 * time.Millisecond)
		Add(1, "Mary", 102)

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

		time.Sleep(1000 * time.Millisecond)

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
