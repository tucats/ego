package proxy

import "github.com/tucats/ego/data"

var ProxyPackage = data.NewPackageFromMap("proxy", map[string]any{
	"Exchange": data.Function{
		Declaration: &data.Declaration{
			Name: "Exchange",
			Parameters: []data.Parameter{
				{
					Name: "method",
					Type: data.StringType,
				},
				{
					Name: "url",
					Type: data.StringType,
				},
				{
					Name: "request",
					Type: data.InterfaceType,
				},
				{
					Name: "response",
					Type: data.PointerType(data.InterfaceType),
				},
			},
			Returns: []*data.Type{data.ErrorType},
		},
		Value: exchange,
	},
})
