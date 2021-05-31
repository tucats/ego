package util

import (
	"bytes"
	"encoding/gob"
)

func RealSizeOf(v interface{}) int {
	b := new(bytes.Buffer)
	if err := gob.NewEncoder(b).Encode(v); err != nil {
		return 0
	}

	return b.Len()
}
