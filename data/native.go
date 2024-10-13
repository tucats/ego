package data

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/errors"
)

const NativeFieldName = "__native"

// SetNative sets the native value of a struct object.
func (s *Struct) SetNative(value interface{}) *Struct {
	_ = s.SetAlways(NativeFieldName, value)

	return s
}

// GetNativeTime retrieves the time value of a native struct object.
func GetNativeUUID(structure interface{}) (uuid.UUID, error) {
	var err error

	if nativeStructure, ok := structure.(*Struct); ok {
		if value, found := nativeStructure.Get(NativeFieldName); found {
			if u, ok := value.(*uuid.UUID); ok {
				return *u, nil
			} else if u, ok := value.(uuid.UUID); ok {
				return u, nil
			} else {
				err = errors.ErrInvalidField.Context("uuid.UUID value")
			}
		} else {
			err = errors.ErrInvalidField.Context("native value")
		}
	} else {
		err = errors.ErrInvalidStruct
	}

	return uuid.Nil, err
}

// GetNativeTime retrieves the time value of a native struct object.
func GetNativeTime(structure interface{}) (*time.Time, error) {
	var err error

	if nativeStructure, ok := structure.(*Struct); ok {
		if value, found := nativeStructure.Get(NativeFieldName); found {
			if timeValue, ok := value.(*time.Time); ok {
				return timeValue, nil
			} else if timeValue, ok := value.(time.Time); ok {
				return &timeValue, nil
			} else {
				err = errors.ErrInvalidField.Context("time.Time value")
			}
		} else {
			err = errors.ErrInvalidField.Context("native value")
		}
	} else {
		err = errors.ErrInvalidStruct
	}

	return nil, err
}

// GetNativeTime retrieves the time value of a native struct object.
func GetNativeDuration(structure interface{}) (*time.Duration, error) {
	var (
		err error
	)

	if nativeStructure, ok := structure.(*Struct); ok {
		if value, found := nativeStructure.Get(NativeFieldName); found {
			if d, ok := value.(*time.Duration); ok {
				return d, nil
			}

			if d, ok := value.(time.Duration); ok {
				return &d, nil
			}

			if d, ok := value.(int64); ok {
				nd := time.Duration(d)

				return &nd, nil
			}

			if d, ok := value.(float64); ok {
				nd := time.Duration(d)

				return &nd, nil
			}

			err = errors.ErrInvalidField.Context("time.Duration value")
		} else {
			err = errors.ErrInvalidField.Context("native value")
		}
	} else {
		err = errors.ErrInvalidStruct
	}

	return nil, err
}

func (s *Struct) FormatNative() string {
	text := "nil"

	if nativeValue, found := s.Get(NativeFieldName); found {
		text = fmt.Sprintf("%v", nativeValue)
	}

	return text
}
