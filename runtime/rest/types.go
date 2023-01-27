package rest

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

// rest.Client type specification.
const restTypeSpec = `
type Client struct {
	client 		interface{},
	baseURL 	string,
	MediaType 	string,
	Response 	string,
	Status 		int,
	verify 		bool,
	Headers 	map[string]string,
}`

var restType *data.Type

func Initialize(s *symbols.SymbolTable) {
	t, _ := compiler.CompileTypeSpec(restTypeSpec)

	t.DefineFunctions(map[string]data.Function{
		"Close": {
			Declaration: &data.FunctionDeclaration{
				Name:         "Close",
				ReceiverType: t,
				ReturnTypes: []*data.Type{
					data.ErrorType,
				},
			},
			Value: Close,
		},

		"Get": {
			Declaration: &data.FunctionDeclaration{
				Name:         "Get",
				ReceiverType: t,
				Parameters: []data.FunctionParameter{
					{
						Name:     "endpoint",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{
					data.ErrorType,
				},
			},
			Value: Get,
		},

		"Post": {
			Declaration: &data.FunctionDeclaration{
				Name:         "Post",
				ReceiverType: t,
				Parameters: []data.FunctionParameter{
					{
						Name:     "endpoint",
						ParmType: data.StringType,
					},
					{
						Name:     "body",
						ParmType: data.InterfaceType,
					},
				},
				ReturnTypes: []*data.Type{
					data.ErrorType,
				},
			},
			Value: Post,
		},

		"Delete": {
			Declaration: &data.FunctionDeclaration{
				Name:         "Delete",
				ReceiverType: t,
				Parameters: []data.FunctionParameter{
					{
						Name:     "endpoint",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{
					data.ErrorType,
				},
			},
			Value: Delete,
		},

		"Base": {
			Declaration: &data.FunctionDeclaration{
				Name:         "Base",
				ReceiverType: t,
				Parameters: []data.FunctionParameter{
					{
						Name:     "url",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{
					t,
				},
			},
			Value: Base,
		},

		"Debug": {
			Declaration: &data.FunctionDeclaration{
				Name:         "Debug",
				ReceiverType: t,
				Parameters: []data.FunctionParameter{
					{
						Name:     "flag",
						ParmType: data.BoolType,
					},
				},
				ReturnTypes: []*data.Type{
					t,
				},
			},
			Value: Debug,
		},

		"Media": {
			Declaration: &data.FunctionDeclaration{
				Name:         "Media",
				ReceiverType: t,
				Parameters: []data.FunctionParameter{
					{
						Name:     "mediaType",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{
					t,
				},
			},
			Value: Media},
		"Token": {
			Declaration: &data.FunctionDeclaration{
				Name:         "Token",
				ReceiverType: t,
				Parameters: []data.FunctionParameter{
					{
						Name:     "tokenString",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{
					t,
				},
			},
			Value: Token,
		},

		"Auth": {
			Declaration: &data.FunctionDeclaration{
				Name:         "Auth",
				ReceiverType: t,
				Parameters: []data.FunctionParameter{
					{
						Name:     "username",
						ParmType: data.StringType,
					},
					{
						Name:     "password",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{
					t,
				},
			},
			Value: Auth,
		},

		"Verify": {
			Declaration: &data.FunctionDeclaration{
				Name:         "Verify",
				ReceiverType: t,
				Parameters: []data.FunctionParameter{
					{
						Name:     "flag",
						ParmType: data.BoolType,
					},
				},
				ReturnTypes: []*data.Type{
					t,
				},
			},
			Value: Verify,
		},

		"Status": {
			Declaration: &data.FunctionDeclaration{
				Name:         "Status",
				ReceiverType: t,
				Parameters: []data.FunctionParameter{
					{
						Name:     "code",
						ParmType: data.IntType,
					},
				},
				ReturnTypes: []*data.Type{
					data.StringType,
				},
			},
			Value: Status,
		},
	})

	restType = t.SetPackage("rest")

	newpkg := data.NewPackageFromMap("rest", map[string]interface{}{
		"New": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name:        "New",
				ReturnTypes: []*data.Type{data.PointerType(restType)},
			},
			Value: New,
		},
		"Status": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Status",
				Parameters: []data.FunctionParameter{
					{
						Name:     "code",
						ParmType: data.IntType,
					},
				},
				ReturnTypes: []*data.Type{data.StringType},
			},
			Value: Status,
		},
		"ParseURL": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "ParseURL",
				Parameters: []data.FunctionParameter{
					{
						Name:     "url",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.MapType(data.StringType, data.InterfaceType)},
			},
			Value: ParseURL,
		},
		"Client": restType,
	}).SetBuiltins(true)

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
