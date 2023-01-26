package errors

import (
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/functions"
	"github.com/tucats/ego/symbols"
)

func InitializeErrors(s *symbols.SymbolTable) {
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

	// Register the (e error) Unwrap( ) interface{} function, which returns
	// the context value associated with the error.
	data.ErrorType.DefineFunction("Unwrap", &data.FunctionDeclaration{
		Name:         "Unwrap",
		ReceiverType: data.ErrorType,
		ReturnTypes:  []*data.Type{data.InterfaceType},
	}, Unwrap)
}
