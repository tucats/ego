package cipher

import (
	"github.com/tucats/ego/data"
)

var CipherAuthType *data.Type = data.TypeDefinition("Token", data.StructType).
	DefineField("Name", data.StringType).
	DefineField("Data", data.StringType).
	DefineField("TokenID", data.StringType).
	DefineField("AuthID", data.StringType).
	DefineField("Expires", data.StringType)

var CipherPackage = data.NewPackageFromMap("cipher", map[string]interface{}{
	"Token": CipherAuthType,
	"Seal": data.Function{
		Declaration: &data.Declaration{
			Name: "Seal",
			Parameters: []data.Parameter{
				{
					Name: "text",
					Type: data.PointerType(data.StringType),
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value: sealString,
	},
	"Unseal": data.Function{
		Declaration: &data.Declaration{
			Name: "Unseal",
			Parameters: []data.Parameter{
				{
					Name: "sealedText",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value: unsealString,
	},
	"New": data.Function{
		Declaration: &data.Declaration{
			Name: "New",
			Parameters: []data.Parameter{
				{
					Name: "name",
					Type: data.StringType,
				},
				{
					Name: "data",
					Type: data.StringType,
				},
				{
					Name: "expiration",
					Type: data.StringType,
				},
			},
			Returns:  []*data.Type{data.StringType},
			ArgCount: data.Range{1, 3},
		},
		Value: NewToken,
	},
	"Decrypt": data.Function{
		Declaration: &data.Declaration{
			Name: "Decrypt",
			Parameters: []data.Parameter{
				{
					Name: "encryptedText",
					Type: data.StringType,
				},
				{
					Name: "key",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType, data.ErrorType},
		},
		Value: decrypt,
	},
	"Encrypt": data.Function{
		Declaration: &data.Declaration{
			Name: "Encrypt",
			Parameters: []data.Parameter{
				{
					Name: "text",
					Type: data.StringType,
				},
				{
					Name: "key",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value: encrypt,
	},
	"Hash": data.Function{
		Declaration: &data.Declaration{
			Name: "Hash",
			Parameters: []data.Parameter{
				{
					Name: "text",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.StringType},
		},
		Value: hash,
	},
	"Random": data.Function{
		Declaration: &data.Declaration{
			Name: "Random",
			Parameters: []data.Parameter{
				{
					Name: "bits",
					Type: data.IntType,
				},
			},
			Returns:  []*data.Type{data.StringType},
			ArgCount: data.Range{0, 1},
		},
		Value: random,
	},
	"Extract": data.Function{
		Declaration: &data.Declaration{
			Name: "Extract",
			Parameters: []data.Parameter{
				{
					Name: "token",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{CipherAuthType},
		},
		Value: Extract,
	},
	"Validate": data.Function{
		Declaration: &data.Declaration{
			Name: "Validate",
			Parameters: []data.Parameter{
				{
					Name: "token",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{data.BoolType},
		},
		Value: Validate,
	},
})
