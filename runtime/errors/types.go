package errors

import (
	"sync"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

var initLock sync.Mutex

func Initialize(s *symbols.SymbolTable) {
	initLock.Lock()
	defer initLock.Unlock()

	if _, found := s.Root().Get("errors"); !found {
		newpkg := data.NewPackageFromMap("errors", map[string]interface{}{
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

		pkg, _ := bytecode.GetPackage(newpkg.Name)
		pkg.Merge(newpkg)
		s.Root().SetAlways(newpkg.Name, newpkg)

		// Register the (e error) Error() function, which generates a formatted string
		// of the error value.
		data.ErrorType.DefineFunction("Error", &data.Declaration{
			Name:     "Error",
			Type:     data.ErrorType,
			Returns:  []*data.Type{data.StringType},
			ArgCount: data.Range{0, 0},
		}, Error)

		// Register the (e error) Is( other error) bool function, which compares
		// the receiver to another error value and returns true if they are equal.
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
		// the context value associated with the error.
		data.ErrorType.DefineFunction("Unwrap", &data.Declaration{
			Name:    "Unwrap",
			Type:    data.ErrorType,
			Returns: []*data.Type{data.InterfaceType},
		}, unwrap)
	}
}
