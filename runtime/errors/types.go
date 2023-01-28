package errors

import (
	"github.com/tucats/ego/builtins"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

func InitializeErrors(s *symbols.SymbolTable) {
	// Register the errors.New function. Unline the Go version, it can accept an optional second
	// argument which is stored with the message as the context (data) value for the error.
	_ = builtins.AddFunction(s, builtins.FunctionDefinition{
		Name: "New",
		Pkg:  "errors",
		Min:  1,
		Max:  2,
		F:    NewError,
		D: &data.Declaration{
			Name: "New",
			Parameters: []data.Parameter{
				{
					Name: "msg",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.ErrorType},
		},
	})

	// Register the (e error) Error() function, which generates a formatted string
	// of the error value.
	data.ErrorType.DefineFunction("Error", &data.Declaration{
		Name:    "Error",
		Type:    data.ErrorType,
		Returns: []*data.Type{data.StringType},
	}, Error)

	// Register the (e error) Is( other error) bool function, which compares
	// the receiver to anotehr error value and returns true if they are equal.
	data.ErrorType.DefineFunction("Is", &data.Declaration{
		Name: "Is",
		Type: data.ErrorType,
		Parameters: []data.Parameter{
			{
				Name: "err",
				Type: data.ErrorType,
			},
		},
		Returns: []*data.Type{data.StringType},
	}, Is)

	// Register the (e error) Unwrap( ) interface{} function, which returns
	// the context value associated with the error.
	data.ErrorType.DefineFunction("Unwrap", &data.Declaration{
		Name:    "Unwrap",
		Type:    data.ErrorType,
		Returns: []*data.Type{data.InterfaceType},
	}, Unwrap)
}
