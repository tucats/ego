package reflect

import (
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
	ArgCount []int
}`

const reflectionTypeDef = `
type Reflection struct {
	name string
	type string
	istype bool
	native bool
	imports bool
	builtins bool
	basetype string
	members []string
	size int
	error error
	text string
	context string
	declaration FunctionDeclaration
}`

var funcParmType, funcDeclType, reflectionType *data.Type

func Initialize(s *symbols.SymbolTable) {
	var err error

	funcParmType, err = compiler.CompileTypeSpec(FunctionParameterTypeDef, nil)
	if err != nil {
		ui.Log(ui.InternalLogger, "Error compiling reflect.FunctionParameter, %v", err)
	} else {
		s.SetAlways("FunctionParameter", funcParmType)
	}

	funcDeclType, err = compiler.CompileTypeSpec(FunctionDeclarationTypeDef, map[string]*data.Type{
		"FunctionParameter": funcParmType,
	})
	if err != nil {
		ui.Log(ui.InternalLogger, "Error compiling reflect.FunctionDeclaration, %v", err)
	} else {
		s.SetAlways("FunctionDeclaration", funcParmType)
	}

	reflectionType, err = compiler.CompileTypeSpec(reflectionTypeDef, map[string]*data.Type{
		"FunctionParameter":   funcParmType,
		"FunctionDeclaration": funcDeclType,
	})
	if err != nil {
		ui.Log(ui.InternalLogger, "Error compiling reflect.Reflection, %v", err)
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
		"Basetype": {
			Declaration: &data.Declaration{
				Name:    "Basetype",
				Type:    reflectionType,
				Returns: []*data.Type{data.StringType},
			},
			Value: getBasetype,
		},
		"Builtins": {
			Declaration: &data.Declaration{
				Name:    "Builtins",
				Type:    reflectionType,
				Returns: []*data.Type{data.StringType},
			},
			Value: getBuiltins,
		},
		"Context": {
			Declaration: &data.Declaration{
				Name:    "Context",
				Type:    reflectionType,
				Returns: []*data.Type{data.StringType},
			},
			Value: getContext,
		},
		"Declaration": {
			Declaration: &data.Declaration{
				Name:    "Declaration",
				Type:    reflectionType,
				Returns: []*data.Type{data.StringType},
			},
			Value: getDeclaration,
		},
		"Error": {
			Declaration: &data.Declaration{
				Name:    "Error",
				Type:    reflectionType,
				Returns: []*data.Type{data.ErrorType},
			},
			Value: getError,
		},
		"Functions": {
			Declaration: &data.Declaration{
				Name:    "Functions",
				Type:    reflectionType,
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: getFunctions,
		},
		"Imports": {
			Declaration: &data.Declaration{
				Name:    "Imports",
				Type:    reflectionType,
				Returns: []*data.Type{data.StringType},
			},
			Value: getImports,
		},

		"IsType": {
			Declaration: &data.Declaration{
				Name:    "IsType",
				Type:    reflectionType,
				Returns: []*data.Type{data.BoolType},
			},
			Value: getIsType,
		},
		"Items": {
			Declaration: &data.Declaration{
				Name:    "Items",
				Type:    reflectionType,
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: getItems,
		},
		"Members": {
			Declaration: &data.Declaration{
				Name:    "Members",
				Type:    reflectionType,
				Returns: []*data.Type{data.ArrayType(data.StringType)},
			},
			Value: getMembers,
		},
		"Name": {
			Declaration: &data.Declaration{
				Name:    "Name",
				Type:    reflectionType,
				Returns: []*data.Type{data.BoolType},
			},
			Value: getName,
		},
		"Native": {
			Declaration: &data.Declaration{
				Name:    "Native",
				Type:    reflectionType,
				Returns: []*data.Type{data.BoolType},
			},
			Value: getNative,
		},
		"Size": {
			Declaration: &data.Declaration{
				Name:    "Size",
				Type:    reflectionType,
				Returns: []*data.Type{data.IntType},
			},
			Value: getSize,
		},
		"Text": {
			Declaration: &data.Declaration{
				Name:    "Text",
				Type:    reflectionType,
				Returns: []*data.Type{data.StringType},
			},
			Value: getText,
		},
		"Type": {
			Declaration: &data.Declaration{
				Name:    "Type",
				Type:    reflectionType,
				Returns: []*data.Type{data.StringType},
			},
			Value: getType,
		},
	})

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
			Declaration: &data.Declaration{
				Name: "Type",
				Parameters: []data.Parameter{
					{
						Name: "any",
						Type: data.InterfaceType,
					},
				},
				Returns: []*data.Type{data.StringType},
			},
			Value: describeType,
		},
	})

	pkg, _ := bytecode.GetPackage(newpkg.Name)
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name, newpkg)
}
