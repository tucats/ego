package db

import (
	"sync"

	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

var rowsType *data.Type
var clientType *data.Type
var typeDefLock sync.Mutex

// db.Client type specification.
const dbTypeSpec = `
type Client struct {
	client 		interface{},
	asStruct 	bool,
	rowCount 	int,
	transaction	interface{},
	constr 		string,
}`

// db.Rows type specification.
const dbRowsTypeSpec = `
type Rows struct {
	client 	interface{},
	rows 	interface{},
	db 		interface{},
}`

const (
	clientFieldName      = "client"
	constrFieldName      = "Constr"
	dbFieldName          = "db"
	rowCountFieldName    = "Rowcount"
	rowsFieldName        = "rows"
	asStructFieldName    = "asStruct"
	transactionFieldName = "transaction"
)

func Initialize(s *symbols.SymbolTable) {
	typeDefLock.Lock()
	defer typeDefLock.Unlock()

	if clientType == nil {
		rowT := initRowsTypeDef()

		t, _ := compiler.CompileTypeSpec(dbTypeSpec)

		t.DefineFunction("Begin", &data.FunctionDeclaration{
			Name:         "Begin",
			ReceiverType: data.PointerType(t),
			ReturnTypes:  []*data.Type{data.ErrorType},
		}, Begin)

		t.DefineFunction("Commit", &data.FunctionDeclaration{
			Name:         "Commit",
			ReceiverType: data.PointerType(t),
			ReturnTypes:  []*data.Type{data.ErrorType},
		}, Commit)

		t.DefineFunction("Rollback", &data.FunctionDeclaration{
			Name:         "Rollback",
			ReceiverType: data.PointerType(t),
			ReturnTypes:  []*data.Type{data.ErrorType},
		}, Rollback)

		t.DefineFunction("Query", &data.FunctionDeclaration{
			Name:         "Query",
			ReceiverType: data.PointerType(t),
			Parameters: []data.FunctionParameter{
				{
					Name:     "sql",
					ParmType: data.StringType,
				},
			},
			ReturnTypes: []*data.Type{
				data.PointerType(rowsType),
				data.ErrorType,
			},
		}, Query)

		t.DefineFunction("QueryResult", &data.FunctionDeclaration{
			Name:         "Execute",
			ReceiverType: data.PointerType(t),
			Parameters: []data.FunctionParameter{
				{
					Name:     "sql",
					ParmType: data.StringType,
				},
			},
			ReturnTypes: []*data.Type{
				data.ArrayType(data.ArrayType(data.InterfaceType)),
				data.ErrorType,
			},
		}, QueryResult)

		t.DefineFunction("Execute", &data.FunctionDeclaration{
			Name:         "Execute",
			ReceiverType: data.PointerType(t),
			Parameters: []data.FunctionParameter{
				{
					Name:     "sql",
					ParmType: data.StringType,
				},
			},
			ReturnTypes: []*data.Type{
				data.IntType,
				data.ErrorType,
			},
		}, Execute)

		t.DefineFunction("Close", &data.FunctionDeclaration{
			Name:         "Close",
			ReceiverType: data.PointerType(t),
			ReturnTypes:  []*data.Type{data.ErrorType},
		}, Close)

		t.DefineFunction("AsStruct", &data.FunctionDeclaration{
			Name:         "AsStruct",
			ReceiverType: data.PointerType(t),
			Parameters: []data.FunctionParameter{
				{
					Name:     "flag",
					ParmType: data.BoolType,
				},
			},
			ReturnTypes: []*data.Type{data.VoidType},
		}, AsStructures)

		clientType = t.SetPackage("db")

		newpkg := data.NewPackageFromMap("db", map[string]interface{}{
			"New":              New,
			"Client":           t,
			"Rows":             rowT,
			data.TypeMDKey:     data.PackageType("db"),
			data.ReadonlyMDKey: true,
		}).SetBuiltins(true)

		pkg, _ := bytecode.GetPackage(newpkg.Name())
		pkg.Merge(newpkg)
		s.Root().SetAlways(newpkg.Name(), newpkg)
	}
}

func initRowsTypeDef() *data.Type {
	t, _ := compiler.CompileTypeSpec(dbRowsTypeSpec)

	t.DefineFunction("Next", &data.FunctionDeclaration{
		Name:         "Next",
		ReceiverType: data.PointerType(t),
		ReturnTypes:  []*data.Type{data.BoolType},
	}, rowsNext)

	t.DefineFunction("Scan", &data.FunctionDeclaration{
		Name: "Scan",
		Parameters: []data.FunctionParameter{
			{
				Name:     "value",
				ParmType: data.PointerType(data.InterfaceType),
			},
		},
		Variadic:     true,
		ReceiverType: data.PointerType(t),
		ReturnTypes:  []*data.Type{data.ErrorType},
	}, rowsScan)

	t.DefineFunction("Close", &data.FunctionDeclaration{
		Name:         "Close",
		ReceiverType: data.PointerType(t),
		ReturnTypes:  []*data.Type{data.ErrorType},
	}, rowsClose)

	t.DefineFunction("Headings", &data.FunctionDeclaration{
		Name:         "Headings",
		ReceiverType: data.PointerType(t),
		ReturnTypes:  []*data.Type{data.ArrayType(data.StringType)},
	}, rowsHeadings)

	rowsType = t.SetPackage("db")

	return rowsType
}
