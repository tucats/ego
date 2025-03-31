package validate

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/util"
)

func (i Item) Validate(item interface{}) error {
	var err error

	switch i.Type {
	case AnyType:
		return nil

	case DurationType:
		value := data.String(item)

		_, err := util.ParseDuration(value)
		if err != nil {
			return errors.ErrValidationError.Clone().Chain(errors.New(err))
		}

	case TimeType:
		// No validation yet for time type

	case UUIDType:
		value := data.String(item)
		// Only validate non-empty UUID values
		if len(value) > 0 {
			_, err := uuid.Parse(value)
			if err != nil {
				return errors.ErrValidationError.Clone().Chain(errors.New(err))
			}
		}

	case IntType:
		value, err := data.Int(item)
		if err != nil {
			return errors.ErrValidationError.Clone().Chain(errors.New(err))
		}

		if float64(value) != data.Float64OrZero(item) {
			return errors.ErrValidationError.Clone().Chain(errors.ErrInvalidInteger.Clone().Context(item))
		}

		if i.HasMin {
			testValue, _ := data.Int(i.Min)
			if value < testValue {
				return errors.ErrValidationError.Clone().Chain(errors.ErrTooSmall.Clone().Context(value))
			}
		}

		if i.HasMax {
			testValue, _ := data.Int(i.Max)
			if value > testValue {
				return errors.ErrValidationError.Clone().Chain(errors.ErrTooLarge.Clone().Context(value))
			}
		}

		if len(i.Enum) > 0 {
			for _, enum := range i.Enum {
				enumValue, _ := data.Int(enum)
				if enumValue == value {
					return nil
				}
			}

			return errors.ErrValidationError.Clone().Chain(errors.ErrInvalidValue.Clone().Context(value))
		}

	case FloatType, NumType:
		value, err := data.Float64(item)
		if err != nil {
			return errors.ErrValidationError.Clone().Chain(errors.New(err))
		}

		if i.HasMin {
			testValue, _ := data.Float64(i.Min)
			if value < testValue {
				return errors.ErrValidationError.Clone().Chain(errors.ErrTooSmall.Clone().Context(value))
			}
		}

		if i.HasMax {
			testValue, _ := data.Float64(i.Max)
			if value > testValue {
				return errors.ErrValidationError.Clone().Chain(errors.ErrTooLarge.Clone().Context(value))
			}
		}

		if len(i.Enum) > 0 {
			for _, enum := range i.Enum {
				enumValue, _ := data.Float64(enum)
				if enumValue == value {
					return nil
				}
			}

			return errors.ErrValidationError.Clone().Chain(errors.ErrInvalidValue.Clone().Context(value))
		}

	case BoolType:
		value := data.String(item)
		if strings.EqualFold(value, "true") || strings.EqualFold(value, "false") {
			return nil
		}

		return errors.ErrValidationError.Clone().Chain(errors.ErrInvalidBooleanValue)

	case StringType:
		value := data.String(item)

		if i.MinLen > 0 && len(value) < i.MinLen {
			return errors.ErrValidationError.Clone().Chain(errors.ErrInvalidValue.Clone().Context(value))
		}

		if i.MaxLen > 0 && len(value) > i.MaxLen {
			return errors.ErrValidationError.Clone().Chain(errors.ErrInvalidValue.Clone().Context(value))
		}

		if i.Required && value == "" {
			return errors.ErrValidationError.Clone().Chain(errors.ErrInvalidValue.Clone().Context("value"))
		}

		if i.HasMin {
			testValue := data.String(i.Min)
			if value < testValue {
				return errors.ErrValidationError.Clone().Chain(errors.ErrTooSmall.Clone().Context(value))
			}
		}

		if i.HasMax {
			testValue := data.String(i.Max)
			if value > testValue {
				return errors.ErrValidationError.Clone().Chain(errors.ErrTooLarge.Clone().Context(value))
			}
		}

		if len(i.Enum) > 0 {
			for _, enum := range i.Enum {
				enumValue := data.String(enum)
				if i.MatchCase {
					if strings.EqualFold(enumValue, value) {
						return nil
					}
				}

				if enumValue == value {
					return nil
				}
			}

			return errors.ErrValidationError.Clone().Chain(errors.ErrInvalidValue.Clone().Context(value))
		}

	default:
		// See if this is a dictionary item name.
		spec := Lookup(i.Type)
		if spec != nil {
			switch r := spec.(type) {
			case Item:
				return r.Validate(item)

			case Object:
				return r.Validate(item)

			case Array:
				return r.Validate(item)

			default:
				ui.Panic("Invalid specification type: " + fmt.Sprintf("%T", r))
			}
		}

		return errors.ErrInvalidType.In("Validate").Clone().Context(i.Type)
	}

	return err
}
