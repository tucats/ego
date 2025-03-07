package errors

import (
	"github.com/tucats/ego/data"
)

var ErrorsPackage = data.NewPackageFromMap("errors", map[string]interface{}{
	// Register the errors.New function. Unline the Go version, it can accept an optional second
	// argument which is stored with the message as the context (data) value for the error.
	"New": data.Function{
		Declaration: &data.Declaration{
			Name:     "New",
			ArgCount: data.Range{1, 2},
			Parameters: []data.Parameter{
				{
					Name: "msg",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.ErrorType},
		},
		Value: newError,
	},
}).Initialize(func(p *data.Package) {
	// Register the (e error) Error() function, which generates a formatted string
	// of the error value. This is registered against the builtin Ego error type,
	// as opposed to being defined as a package type here.
	data.ErrorType.DefineFunction("Error", &data.Declaration{
		Name:     "Error",
		Type:     data.ErrorType,
		Returns:  []*data.Type{data.StringType},
		ArgCount: data.Range{0, 0},
	}, Error)

	// Register the (e error) Is( other error) bool function, which compares
	// the receiver to another error value and returns true if they are equal.
	// This is registered against the builtin Ego error type,
	// as opposed to being defined as a package type here.
	data.ErrorType.DefineFunction("Is", &data.Declaration{
		Name: "Is",
		Type: data.ErrorType,
		Parameters: []data.Parameter{
			{
				Name: "err",
				Type: data.ErrorType,
			},
		},
		ArgCount: data.Range{1, 1},
		Returns:  []*data.Type{data.StringType},
	}, isError)

	// Register the (e error) Unwrap( ) interface{} function, which returns
	// the context value associated with the error. This is registered against
	// the builtin Ego error type, as opposed to being defined as a package type here.
	data.ErrorType.DefineFunction("Unwrap", &data.Declaration{
		Name:    "Unwrap",
		Type:    data.ErrorType,
		Returns: []*data.Type{data.InterfaceType},
	}, unwrap)

	data.ErrorType.DefineFunction("Context", &data.Declaration{
		Name: "Context",
		Parameters: []data.Parameter{
			{
				Name: "v",
				Type: data.PointerType(data.InterfaceType),
			},
		},
		Type:    data.ErrorType,
		Returns: []*data.Type{data.ErrorType},
	}, context)

	data.ErrorType.DefineFunction("At", &data.Declaration{
		Name: "At",
		Parameters: []data.Parameter{
			{
				Name: "line",
				Type: data.PointerType(data.IntType),
			},
		},
		Type:    data.ErrorType,
		Returns: []*data.Type{data.ErrorType},
	}, at)

	data.ErrorType.DefineFunction("In", &data.Declaration{
		Name: "In",
		Parameters: []data.Parameter{
			{
				Name: "name",
				Type: data.PointerType(data.StringType),
			},
		},
		Type:    data.ErrorType,
		Returns: []*data.Type{data.ErrorType},
	}, in)
})

var ErorsPackage = data.NewPackageFromMap("errors", map[string]interface{}{
	// Register the errors.New function. Unline the Go version, it can accept an optional second
	// argument which is stored with the message as the context (data) value for the error.
	"New": data.Function{
		Declaration: &data.Declaration{
			Name:     "New",
			ArgCount: data.Range{1, 2},
			Parameters: []data.Parameter{
				{
					Name: "msg",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.ErrorType},
		},
		Value: newError,
	},
})
