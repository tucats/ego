package data

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/errors"
)

// NativeFieldName is the key used to store the Go-native value inside an Ego
// Struct wrapper.  Some Ego packages (time, uuid, sync, …) wrap Go-native
// objects in a Struct so that Ego code can hold and pass them around.  The
// actual native Go value is stored under this field name, while the other
// fields of the struct hold whatever Ego-visible attributes the package
// chooses to expose.
const NativeFieldName = "__native"

// SetNative stores value as the embedded Go-native object inside the struct.
// It calls SetAlways so that the write bypasses any readonly or static checks
// on the struct — the native field is always writable by the runtime.
// The function returns the receiver (s) so calls can be chained.
func (s *Struct) SetNative(value any) *Struct {
	_ = s.SetAlways(NativeFieldName, value)

	return s
}

// GetNativeUUID extracts a uuid.UUID from an Ego struct that wraps a native
// UUID value.  The struct must have been created by a package that stores a
// *uuid.UUID or uuid.UUID under the NativeFieldName key.
//
// The function handles two cases for the stored value:
//   - *uuid.UUID — a pointer to a UUID; the value is dereferenced before returning.
//   - uuid.UUID  — a value copy of a UUID; returned directly.
//
// Returns uuid.Nil and an error if structure is not an Ego *Struct, if the
// NativeFieldName field is missing, or if the stored value is not a UUID.
func GetNativeUUID(structure any) (uuid.UUID, error) {
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

// GetNativeTime extracts a *time.Time from a value that is either:
//   - a *time.Time already,
//   - a time.Time value (which is wrapped in a pointer before returning),
//   - an Ego *Struct containing a *time.Time or time.Time under NativeFieldName.
//
// This flexibility lets callers pass either a raw Go time or an Ego struct
// without needing to know which form was used.
func GetNativeTime(structure any) (*time.Time, error) {
	var err error

	// Fast path: already a *time.Time.
	if t, ok := structure.(*time.Time); ok {
		return t, nil
	}

	// Fast path: a bare time.Time value — take its address so we can return
	// a pointer without the caller needing to deal with copying.
	if t, ok := structure.(time.Time); ok {
		return &t, nil
	}

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

// GetNativeDuration extracts a *time.Duration from a value that may be:
//   - a *time.Duration,
//   - a time.Duration value,
//   - an Ego *Struct whose NativeFieldName field holds a *time.Duration,
//     time.Duration, int64, or float64.
//
// The int64 and float64 cases arise because Ego sometimes stores duration
// values as plain integers (nanoseconds since Go's time.Duration is really
// just an int64 underneath).
func GetNativeDuration(structure any) (*time.Duration, error) {
	var (
		err error
	)

	// Fast paths for raw duration values.
	if t, ok := structure.(*time.Duration); ok {
		return t, nil
	}

	if t, ok := structure.(time.Duration); ok {
		return &t, nil
	}

	if nativeStructure, ok := structure.(*Struct); ok {
		if value, found := nativeStructure.Get(NativeFieldName); found {
			if d, ok := value.(*time.Duration); ok {
				return d, nil
			}

			if d, ok := value.(time.Duration); ok {
				return &d, nil
			}

			// int64 and float64 are treated as nanosecond counts, matching
			// Go's underlying representation of time.Duration.
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

// FormatNative returns a human-readable string for the Go-native object
// embedded in a struct.  fmt.Sprintf with "%v" produces Go's default
// representation for any type, which is usually good enough for diagnostics.
// If the native field is missing, "nil" is returned.
func (s *Struct) FormatNative() string {
	text := "nil"

	if nativeValue, found := s.Get(NativeFieldName); found {
		text = fmt.Sprintf("%v", nativeValue)
	}

	return text
}
