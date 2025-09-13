package rest

import (
	"github.com/tucats/ego/data"
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
						data.ErrorType,
					},
					ArgCount: data.Range{1, 2},
				},
				Value: doPost,
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
					},
					Returns: []*data.Type{
						data.ErrorType,
					},
				},
				Value: doDelete,
			},

			"Status": {
				Declaration: &data.Declaration{
					Name: "Status",
					Type: data.OwnType,
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
			Returns:  []*data.Type{data.PointerType(RestClientType)},
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
	"Client": RestClientType,
})
