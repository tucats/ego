package runtime

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
)

// Internal flag indicating if we want error messages to include module and
// line numbers. Default is nope.
var verbose bool = false

func initializeErrors(s *symbols.SymbolTable) {
	// Register the errors.New function. Unline the Go version, it can accept an optional second
	// argument which is stored with the message as the context (data) value for the error.
	_ = functions.AddFunction(s, functions.FunctionDefinition{
		Name: "New",
		Pkg:  "errors",
		Min:  1,
		Max:  2,
		F:    NewError,
		D: &data.FunctionDeclaration{
			Name: "New",
			Parameters: []data.FunctionParameter{
				{
					Name:     "msg",
					ParmType: data.StringType,
				},
			},
			ReturnTypes: []*data.Type{data.ErrorType},
		},
	})

	// Register the (e error) Error() function, which generates a formatted string
	// of the error value.
	data.ErrorType.DefineFunction("Error", &data.FunctionDeclaration{
		Name:         "Error",
		ReceiverType: data.ErrorType,
		ReturnTypes:  []*data.Type{data.StringType},
	}, Error)

	// Register the (e error) Is( other error) bool function, which compares
	// the receiver to anotehr error value and returns true if they are equal.
	data.ErrorType.DefineFunction("Is", &data.FunctionDeclaration{
		Name:         "Is",
		ReceiverType: data.ErrorType,
		Parameters: []data.FunctionParameter{
			{
				Name:     "err",
				ParmType: data.ErrorType,
			},
		},
		ReturnTypes: []*data.Type{data.StringType},
	}, Is)

	// Register the (e error) Is( other error) bool function, which compares
	// the receiver to anotehr error value and returns true if they are equal.
	data.ErrorType.DefineFunction("Unwrap", &data.FunctionDeclaration{
		Name:         "Unwrap",
		ReceiverType: data.ErrorType,
		ReturnTypes:  []*data.Type{data.InterfaceType},
	}, Unwrap)
}

// Error implements the (e error) Error() method for Ego errors.
func Error(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount.In("Error()")
	}

	if v, found := s.Get(defs.ThisVariable); found {
		if e, ok := v.(*errors.Error); ok {
			return e.Error(), nil
		}

		if e, ok := v.(error); ok {
			return e.Error(), nil
		}

		return nil, errors.ErrInvalidType.Context(data.TypeOf(v))
	}

	return nil, errors.ErrNoFunctionReceiver
}

// Is implements the (e error) Is() method for Ego errors.
func Is(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) != 0 {
		return nil, errors.ErrArgumentCount.In("Error()")
	}

	var test error

	if e, ok := args[0].(*errors.Error); ok {
		test = e
	}

	if v, found := s.Get(defs.ThisVariable); found {
		if e, ok := v.(*errors.Error); ok {
			return e.Is(test), nil
		}

		return nil, errors.ErrInvalidType.Context(data.TypeOf(v)).In("Is()")
	}

	return nil, errors.ErrNoFunctionReceiver
}

// Unwrap implements the (e error) Unwrap() method for Ego errors.
func Unwrap(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) > 0 {
		return nil, errors.ErrArgumentCount.In("Error()")
	}

	if v, found := s.Get(defs.ThisVariable); found {
		if e, ok := v.(*errors.Error); ok {
			return e.GetContext(), nil
		}

		if _, ok := v.(error); ok {
			return nil, nil
		}

		return nil, errors.ErrInvalidType.Context(data.TypeOf(v))
	}

	return nil, errors.ErrNoFunctionReceiver
}

func NewError(s *symbols.SymbolTable, args []interface{}) (interface{}, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, errors.ErrArgumentCount.In("New()")
	}

	result := errors.NewMessage(data.String(args[0]))

	if len(args) > 1 {
		_ = result.Context(args[1])
	}

	if verbose {
		if module, found := s.Get(defs.ModuleVariable); found {
			_ = result.In(data.String(module))
		}

		if line, found := s.Get(defs.LineVariable); found {
			_ = result.At(data.Int(line), 0)
		}
	}

	return result, nil
}
