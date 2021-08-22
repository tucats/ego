package util

import (
	"bytes"
	"encoding/gob"
)

func RealSizeOf(v interface{}) int {

	size := 0

	switch v.(type) {

	case bool:
		size = 1
	case byte:
		size = 1
	case int32:
		size = 4
	case int:
		size = 4
	case int64:
		size = 8
	case float32:
		size = 4
	case float64:
		size = 8
	default:
		b := new(bytes.Buffer)
		if err := gob.NewEncoder(b).Encode(v); err != nil {
			return 0
		}
		size = b.Len()
	}

	return size
}
