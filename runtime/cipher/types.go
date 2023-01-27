package cipher

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

var authTypeDef = `
type Token struct {
	Name string
	Data string
	TokenID string
	AuthID string
	Expires string
}`

var authType *data.Type

func Initialize(s *symbols.SymbolTable) {
	authType, _ = compiler.CompileTypeSpec(authTypeDef)

	newpkg := data.NewPackageFromMap("cipher", map[string]interface{}{
		"Token": authType,
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
				},
				Returns:  []*data.Type{data.StringType},
				ArgCount: data.Range{1, 2},
			},
			Value: New,
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
			Value: Decrypt,
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
			Value: Encrypt,
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
			Value: Hash,
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
			Value: Random,
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
				Returns: []*data.Type{authType},
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
	}).SetBuiltins(true)

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
