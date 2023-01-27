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
			Declaration: &data.Declaration{
				Name: "Close",
				Type: t,
				Returns: []*data.Type{
					data.ErrorType,
				},
			},
			Value: Close,
		},

		"Get": {
			Declaration: &data.Declaration{
				Name: "Get",
				Type: t,
				Parameters: []data.Parameter{
					{
						Name: "endpoint",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{
					data.ErrorType,
				},
			},
			Value: Get,
		},

		"Post": {
			Declaration: &data.Declaration{
				Name: "Post",
				Type: t,
				Parameters: []data.Parameter{
					{
						Name: "endpoint",
						Type: data.StringType,
					},
					{
						Name: "body",
						Type: data.InterfaceType,
					},
				},
				Returns: []*data.Type{
					data.ErrorType,
				},
			},
			Value: Post,
		},

		"Delete": {
			Declaration: &data.Declaration{
				Name: "Delete",
				Type: t,
				Parameters: []data.Parameter{
					{
						Name: "endpoint",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{
					data.ErrorType,
				},
			},
			Value: Delete,
		},

		"Base": {
			Declaration: &data.Declaration{
				Name: "Base",
				Type: t,
				Parameters: []data.Parameter{
					{
						Name: "url",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{
					t,
				},
			},
			Value: Base,
		},

		"Debug": {
			Declaration: &data.Declaration{
				Name: "Debug",
				Type: t,
				Parameters: []data.Parameter{
					{
						Name: "flag",
						Type: data.BoolType,
					},
				},
				Returns: []*data.Type{
					t,
				},
			},
			Value: Debug,
		},

		"Media": {
			Declaration: &data.Declaration{
				Name: "Media",
				Type: t,
				Parameters: []data.Parameter{
					{
						Name: "mediaType",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{
					t,
				},
			},
			Value: Media},
		"Token": {
			Declaration: &data.Declaration{
				Name: "Token",
				Type: t,
				Parameters: []data.Parameter{
					{
						Name: "tokenString",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{
					t,
				},
			},
			Value: Token,
		},

		"Auth": {
			Declaration: &data.Declaration{
				Name: "Auth",
				Type: t,
				Parameters: []data.Parameter{
					{
						Name: "username",
						Type: data.StringType,
					},
					{
						Name: "password",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{
					t,
				},
			},
			Value: Auth,
		},

		"Verify": {
			Declaration: &data.Declaration{
				Name: "Verify",
				Type: t,
				Parameters: []data.Parameter{
					{
						Name: "flag",
						Type: data.BoolType,
					},
				},
				Returns: []*data.Type{
					t,
				},
			},
			Value: Verify,
		},

		"Status": {
			Declaration: &data.Declaration{
				Name: "Status",
				Type: t,
				Parameters: []data.Parameter{
					{
						Name: "code",
						Type: data.IntType,
					},
				},
				Returns: []*data.Type{
					data.StringType,
				},
			},
			Value: Status,
		},
	})

	restType = t.SetPackage("rest")

	newpkg := data.NewPackageFromMap("rest", map[string]interface{}{
		"New": data.Function{
			Declaration: &data.Declaration{
				Name:    "New",
				Returns: []*data.Type{data.PointerType(restType)},
			},
			Value: New,
		},
		"Status": data.Function{
			Declaration: &data.Declaration{
				Name: "Status",
				Parameters: []data.Parameter{
					{
						Name: "code",
						Type: data.IntType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: Status,
		},
		"ParseURL": data.Function{
			Declaration: &data.Declaration{
				Name: "ParseURL",
				Parameters: []data.Parameter{
					{
						Name: "url",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{data.MapType(data.StringType, data.InterfaceType)},
			},
			Value: ParseURL,
		},
		"Client": restType,
	}).SetBuiltins(true)

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
