package rest

import (
	"github.com/tucats/ego/internal/language/data"
)

var RestClientType = data.TypeDefinition("Client",
	data.StructureType().
		SetPackage("rest").
		DefineField("client", data.InterfaceType).
		DefineField("baseURL", data.StringType).
		DefineField("MediaType", data.StringType).
		DefineField("Response", data.StringType).
		DefineField("Status", data.IntType).
		DefineField("verify", data.BoolType).
		DefineField("Headers", data.MapType(data.StringType, data.StringType)).
		DefineField("Cookies", data.ArrayType(data.InterfaceType)).
		DefineFunctions(map[string]data.Function{
			"Base": {
				Declaration: &data.Declaration{
					Name: "Base",
					Parameters: []data.Parameter{
						{
							Name: "url",
							Type: data.StringType,
						},
					},
					Returns: []*data.Type{data.OwnType},
				},
				Value: setBase,
			},

			"Debug": {
				Declaration: &data.Declaration{
					Name: "Debug",
					Parameters: []data.Parameter{
						{
							Name: "flag",
							Type: data.BoolType,
						},
					},
					Returns: []*data.Type{data.OwnType},
				},
				Value: setDebug,
			},

			"Media": {
				Declaration: &data.Declaration{
					Name: "Media",
					Parameters: []data.Parameter{
						{
							Name: "mediaType",
							Type: data.StringType,
						},
					},
					Returns: []*data.Type{data.OwnType},
				},
				Value: setMedia,
			},
			"Token": {
				Declaration: &data.Declaration{
					Name: "Token",
					Parameters: []data.Parameter{
						{
							Name: "tokenString",
							Type: data.StringType,
						},
					},
					Returns: []*data.Type{data.OwnType},
				},
				Value: setToken,
			},

			"Auth": {
				Declaration: &data.Declaration{
					Name: "Auth",
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
					Returns: []*data.Type{data.OwnType},
				},
				Value: setAuthentication,
			},

			"Verify": {
				Declaration: &data.Declaration{
					Name: "Verify",
					Parameters: []data.Parameter{
						{
							Name: "flag",
							Type: data.BoolType,
						},
					},
					Returns: []*data.Type{data.OwnType},
				},
				Value: setVerify,
			},

			"UseToken": {
				Declaration: &data.Declaration{
					Name: "UseToken",
					Parameters: []data.Parameter{
						{
							Name: "flag",
							Type: data.BoolType,
						},
					},
					Returns: []*data.Type{data.OwnType, data.ErrorType},
				},
				Value: setUseToken,
			},
		}).
		DefineFunctions(map[string]data.Function{
			"Close": {
				Declaration: &data.Declaration{
					Name: "Close",
					Type: data.OwnType,
					Returns: []*data.Type{
						data.ErrorType,
					},
				},
				Value: closeClient,
			},

			"Get": {
				Declaration: &data.Declaration{
					Name: "Get",
					Type: data.OwnType,
					Parameters: []data.Parameter{
						{
							Name: "endpoint",
							Type: data.StringType,
						},
					},
					Returns: []*data.Type{
						data.InterfaceType,
						data.ErrorType,
					},
				},
				Value: doGet,
			},

			"Post": {
				Declaration: &data.Declaration{
					Name: "Post",
					Type: data.OwnType,
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
						data.InterfaceType,
						data.ErrorType,
					},
					ArgCount: data.Range{1, 2},
				},
				Value: doPost,
			},

			"Put": {
				Declaration: &data.Declaration{
					Name: "Put",
					Type: data.OwnType,
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
						data.InterfaceType,
						data.ErrorType,
					},
					ArgCount: data.Range{1, 2},
				},
				Value: doPut,
			},

			"Patch": {
				Declaration: &data.Declaration{
					Name: "Patch",
					Type: data.OwnType,
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
						data.InterfaceType,
						data.ErrorType,
					},
					ArgCount: data.Range{1, 2},
				},
				Value: doPatch,
			},

			"Delete": {
				Declaration: &data.Declaration{
					Name: "Delete",
					Type: data.OwnType,
					Parameters: []data.Parameter{
						{
							Name: "endpoint",
							Type: data.StringType,
						},
						{
							// Optional: doDelete already reads this
							// (see methods.go) but until this field was
							// added, the declaration only allowed exactly
							// one argument, so a body could never actually
							// be passed from Ego -- see the commit message
							// for the full explanation.
							Name: "body",
							Type: data.InterfaceType,
						},
					},
					Returns: []*data.Type{
						data.InterfaceType,
						data.ErrorType,
					},
					ArgCount: data.Range{1, 2},
				},
				Value: doDelete,
			},
		}),
).SetPackage("rest").FixSelfReferences()

var RestPackage = data.NewPackageFromMap("rest", map[string]any{
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
			Returns:  []*data.Type{data.PointerType(RestClientType), data.ErrorType},
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
			Returns:  []*data.Type{data.MapType(data.StringType, data.InterfaceType), data.ErrorType},
		},
		Value: ParseURL,
	},
	"Client": RestClientType,
})
