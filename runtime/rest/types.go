package rest

import (
	"sync"

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
var initLock sync.Mutex

func Initialize(s *symbols.SymbolTable) {
	initLock.Lock()
	defer initLock.Unlock()

	if restType != nil {
		return
	}

	t, _ := compiler.CompileTypeSpec(restTypeSpec, nil)

	t.DefineFunctions(map[string]data.Function{
		"Close": {
			Declaration: &data.Declaration{
				Name: "Close",
				Type: t,
				Returns: []*data.Type{
					data.ErrorType,
				},
			},
			Value: closeClient,
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
			Value: doGet,
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
				ArgCount: data.Range{1, 2},
			},
			Value: doPost,
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
			Value: doDelete,
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
			Value: setBase,
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
			Value: setDebug,
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
			Value: setMedia,
		},
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
			Value: setToken,
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
			Value: setAuthentication,
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
			Value: setVerify,
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
				Name: "New",
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
				Returns:  []*data.Type{data.PointerType(restType)},
				ArgCount: data.Range{0, 2},
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
					{
						Name: "template",
						Type: data.StringType,
					},
				},
				ArgCount: data.Range{1, 2},
				Returns:  []*data.Type{data.MapType(data.StringType, data.InterfaceType)},
			},
			Value: ParseURL,
		},
		"Client": restType,
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name)
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name, newpkg)
}
