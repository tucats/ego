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
			Declaration: &data.FunctionDeclaration{
				Name: "New",
				Parameters: []data.FunctionParameter{
					{
						Name:     "name",
						ParmType: data.StringType,
					},
					{
						Name:     "data",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.StringType},
				ArgCount:    data.Range{1, 2},
			},
			Value: New,
		},
		"Decrypt": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Decrypt",
				Parameters: []data.FunctionParameter{
					{
						Name:     "encryptedText",
						ParmType: data.StringType,
					},
					{
						Name:     "key",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.StringType, data.ErrorType},
			},
			Value: Decrypt,
		},
		"Encrypt": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Encrypt",
				Parameters: []data.FunctionParameter{
					{
						Name:     "text",
						ParmType: data.StringType,
					},
					{
						Name:     "key",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.StringType},
			},
			Value: Encrypt,
		},
		"Hash": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Hash",
				Parameters: []data.FunctionParameter{
					{
						Name:     "text",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.StringType},
			},
			Value: Hash,
		},
		"Random": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Random",
				Parameters: []data.FunctionParameter{
					{
						Name:     "bits",
						ParmType: data.IntType,
					},
				},
				ReturnTypes: []*data.Type{data.StringType},
				ArgCount:    data.Range{0, 1},
			},
			Value: Random,
		},
		"Extract": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Extract",
				Parameters: []data.FunctionParameter{
					{
						Name:     "token",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{authType},
			},
			Value: Extract,
		},
		"Validate": data.Function{
			Declaration: &data.FunctionDeclaration{
				Name: "Validate",
				Parameters: []data.FunctionParameter{
					{
						Name:     "token",
						ParmType: data.StringType,
					},
				},
				ReturnTypes: []*data.Type{data.BoolType},
			},
			Value: Validate,
		},
	}).SetBuiltins(true)

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}
