package datatypes

import (
	"github.com/tucats/ego/errors"
)

func AddressOf(v interface{}) (interface{}, error) {
	switch actual := v.(type) {
	case bool:
		return &actual, nil
	case byte:
		return &actual, nil
	case int32:
		return &actual, nil
	case int:
		return &actual, nil
	case int64:
		return &actual, nil
	case float32:
		return &actual, nil
	case float64:
		return &actual, nil
	case string:
		return &actual, nil
	case EgoPackage:
		return &actual, nil
	case *EgoStruct:
		return &actual, nil
	case *EgoMap:
		return &actual, nil
	case *EgoArray:
		return &actual, nil
	case *Channel:
		return &actual, nil
	default:
		return &v, nil
	}
}

func Dereference(v interface{}) (interface{}, error) {
	switch actual := v.(type) {
	case *interface{}:
		return *actual, nil
	case *bool:
		return *actual, nil
	case *byte:
		return *actual, nil
	case *int32:
		return *actual, nil
	case *int:
		return *actual, nil
	case *int64:
		return *actual, nil
	case *float32:
		return *actual, nil
	case *float64:
		return *actual, nil
	case *string:
		return *actual, nil
	case *EgoPackage:
		return *actual, nil
	case **EgoMap:
		return *actual, nil
	case **EgoArray:
		return *actual, nil
	case **Channel:
		return *actual, nil

	default:
		return nil, errors.EgoError(errors.ErrNotAPointer)
	}
}
