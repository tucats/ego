package reflect

import (
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

var MaxDeepCopyDepth int = 100

const FunctionParameterTypeDef = `
type FunctionParameter struct {
	Name string
	Type string
}`

const FunctionDeclarationTypeDef = `
type FunctionDeclaration struct {
	Name string
	Parameters []FunctionParameter
	Returns []string
	Argcount []int
}`

const reflectionTypeDef = `
type Reflection struct {
	Name string
	Type type
	Istype bool
	Native bool
	Imports bool
	Builtins bool
	Basetype string
	Members []string
	Size int
	Error error
	Text string
	Context string
	Declaration FunctionDeclaration
}`

var funcParmType, funcDeclType, reflectionType *data.Type
var initLock sync.Mutex

func Initialize(s *symbols.SymbolTable) {
	var err error

	initLock.Lock()
	defer initLock.Unlock()

	if funcParmType == nil {
		funcParmType, err = compiler.CompileTypeSpec(FunctionParameterTypeDef, nil)
		if err != nil {
			ui.Log(ui.InternalLogger, "runtime.compile.error", ui.A{
				"desc":  "reflect.FunctionParameter",
				"error": err})
		} else {
			s.SetAlways("FunctionParameter", funcParmType)
		}

		funcDeclType, err = compiler.CompileTypeSpec(FunctionDeclarationTypeDef, map[string]*data.Type{
			"FunctionParameter": funcParmType,
		})
		if err != nil {
			ui.Log(ui.InternalLogger, "runtime.compile.error", ui.A{
				"desc":  "reflect.FunctionDeclaration",
				"error": err})
		} else {
			s.SetAlways("FunctionDeclaration", funcParmType)
		}

		reflectionType, err = compiler.CompileTypeSpec(reflectionTypeDef, map[string]*data.Type{
			"FunctionParameter":   funcParmType,
			"FunctionDeclaration": funcDeclType,
		})
		if err != nil {
			ui.Log(ui.InternalLogger, "runtime.compile.error", ui.A{
				"desc":  "reflect.Reflection",
				"error": err})
		} else {
			s.SetAlways("Reflection", funcParmType)
		}

		reflectionType.DefineFunctions(map[string]data.Function{
			"String": {
				Declaration: &data.Declaration{
					Name:    "String",
					Type:    reflectionType,
					Returns: []*data.Type{data.StringType},
				},
				Value: getString,
			},
		})
	}

	if _, found := s.Root().Get("reflect"); !found {
		newpkg := data.NewPackageFromMap("reflect", map[string]interface{}{
			"FunctionParameter":   funcParmType.SetPackage("reflect"),
			"FunctionDeclaration": funcDeclType.SetPackage("reflect"),
			"Reflection":          reflectionType.SetPackage("reflect"),
			"DeepCopy": data.Function{
				Declaration: &data.Declaration{
					Name: "DeepCopy",
					Parameters: []data.Parameter{
						{
							Name: "any",
							Type: data.InterfaceType,
						},
						{
							Name: "depth",
							Type: data.IntType,
						},
					},
					ArgCount: data.Range{1, 2},
					Returns:  []*data.Type{data.InterfaceType},
				},
				Value: deepCopy,
			},
			"InstanceOf": data.Function{
				Declaration: &data.Declaration{
					Name: "InstanceOf",
					Parameters: []data.Parameter{
						{
							Name: "any",
							Type: data.InterfaceType,
						},
					},
					Returns: []*data.Type{data.InterfaceType},
				},
				Value: instanceOf,
			},
			"Members": data.Function{
				Declaration: &data.Declaration{
					Name: "Members",
					Parameters: []data.Parameter{
						{
							Name: "any",
							Type: data.InterfaceType,
						},
					},
					Returns: []*data.Type{data.ArrayType(data.StringType)},
				},
				Value: members,
			},
			"Reflect": data.Function{
				Declaration: &data.Declaration{
					Name: "Reflect",
					Parameters: []data.Parameter{
						{
							Name: "any",
							Type: data.InterfaceType,
						},
					},
					Returns: []*data.Type{data.StructType},
				},
				Value: describe,
			},
			"Type": data.Function{
				Extension: true,
				Declaration: &data.Declaration{
					Name: "Type",
					Parameters: []data.Parameter{
						{
							Name: "any",
							Type: data.InterfaceType,
						},
					},
					Returns: []*data.Type{data.TypeType},
				},
				Value: describeType,
			},
		})

		pkg, _ := bytecode.GetPackage(newpkg.Name)
		pkg.Merge(newpkg)
		s.Root().SetAlways(newpkg.Name, newpkg)
	}
}
