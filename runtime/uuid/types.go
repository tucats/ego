package uuid

import (
	"sync"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

var uuidTypeDef *data.Type
var initLock sync.Mutex

func Initialize(s *symbols.SymbolTable) {
	initLock.Lock()
	defer initLock.Unlock()

	if uuidTypeDef != nil {
		return
	}

	// Create the UUID type
	uuidTypeDef = data.NewType("UUID", data.StructKind).SetNativeName("uuid.UUID").SetPackage("uuid")

	// Define the UUID type methods. Since these reference the type in parameters and returns value types,
	// the uuidTypeDef must have already been created before defining the methods.
	uuidTypeDef.DefineFunctions(map[string]data.Function{
		"String": {
			Declaration: &data.Declaration{
				Name:    "String",
				Type:    uuidTypeDef,
				Returns: []*data.Type{data.StringType},
			},
			Value: toString,
		},
		"Gibberish": {
			Declaration: &data.Declaration{
				Name:    "Gibberish",
				Type:    uuidTypeDef,
				Returns: []*data.Type{data.StringType},
			},
			Value: toGibberish,
		},
	})

	newpkg := data.NewPackageFromMap("uuid", map[string]interface{}{
		"New": data.Function{
			Declaration: &data.Declaration{
				Name:    "New",
				Returns: []*data.Type{uuidTypeDef},
			},
			Value: newUUID,
		},
		"Nil": data.Function{
			Declaration: &data.Declaration{
				Name:    "Nil",
				Returns: []*data.Type{uuidTypeDef},
			},
			Value: nilUUID,
		},
		"Parse": data.Function{
			Declaration: &data.Declaration{
				Name: "Parse",
				Parameters: []data.Parameter{
					{
						Name: "text",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{uuidTypeDef, data.ErrorType},
			},
			Value: parseUUID,
		},
		"UUID": uuidTypeDef,
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name)
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name, newpkg)
}
