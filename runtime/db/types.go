package db

import (
	"github.com/tucats/ego/bytecode"
	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/symbols"
)

var rowsType *data.Type
var clientType *data.Type

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
	rowT := initRowsTypeDef()

	t, _ := compiler.CompileTypeSpec(dbTypeSpec ,nil)

	t.DefineFunction("Begin", &data.Declaration{
		Name:    "Begin",
		Type:    data.PointerType(t),
		Returns: []*data.Type{data.ErrorType},
	}, begin)

	t.DefineFunction("Commit", &data.Declaration{
		Name:    "Commit",
		Type:    data.PointerType(t),
		Returns: []*data.Type{data.ErrorType},
	}, commit)

	t.DefineFunction("Rollback", &data.Declaration{
		Name:    "Rollback",
		Type:    data.PointerType(t),
		Returns: []*data.Type{data.ErrorType},
	}, rollback)

	t.DefineFunction("Query", &data.Declaration{
		Name: "Query",
		Type: data.PointerType(t),
		Parameters: []data.Parameter{
			{
				Name: "sql",
				Type: data.StringType,
			},
		},
		Returns: []*data.Type{
			data.PointerType(rowsType),
			data.ErrorType,
		},
	}, query)

	t.DefineFunction("QueryResult", &data.Declaration{
		Name: "Execute",
		Type: data.PointerType(t),
		Parameters: []data.Parameter{
			{
				Name: "sql",
				Type: data.StringType,
			},
		},
		Returns: []*data.Type{
			data.ArrayType(data.ArrayType(data.InterfaceType)),
			data.ErrorType,
		},
	}, queryResult)

	t.DefineFunction("Execute", &data.Declaration{
		Name: "Execute",
		Type: data.PointerType(t),
		Parameters: []data.Parameter{
			{
				Name: "sql",
				Type: data.StringType,
			},
		},
		Returns: []*data.Type{
			data.IntType,
			data.ErrorType,
		},
	}, execute)

	t.DefineFunction("Close", &data.Declaration{
		Name:    "Close",
		Type:    data.PointerType(t),
		Returns: []*data.Type{data.ErrorType},
	}, closeConnection)

	t.DefineFunction("AsStruct", &data.Declaration{
		Name: "AsStruct",
		Type: data.PointerType(t),
		Parameters: []data.Parameter{
			{
				Name: "flag",
				Type: data.BoolType,
			},
		},
		Returns: []*data.Type{data.VoidType},
	}, asStructures)

	clientType = t.SetPackage("db")

	newpkg := data.NewPackageFromMap("db", map[string]interface{}{
		"New": data.Function{
			Declaration: &data.Declaration{
				Name: "New",
				Parameters: []data.Parameter{
					{
						Name: "connection",
						Type: data.StringType,
					},
				},
				Returns: []*data.Type{t},
			},
			Value: newConnection,
		},
		"Client":           t,
		"Rows":             rowT,
		data.TypeMDKey:     data.PackageType("db"),
		data.ReadonlyMDKey: true,
	}).SetBuiltins(true)

	pkg, _ := bytecode.GetPackage(newpkg.Name())
	pkg.Merge(newpkg)
	s.Root().SetAlways(newpkg.Name(), newpkg)
}

func initRowsTypeDef() *data.Type {
	t, _ := compiler.CompileTypeSpec(dbRowsTypeSpec ,nil)

	t.DefineFunction("Next", &data.Declaration{
		Name:    "Next",
		Type:    data.PointerType(t),
		Returns: []*data.Type{data.BoolType},
	}, rowsNext)

	t.DefineFunction("Scan", &data.Declaration{
		Name: "Scan",
		Parameters: []data.Parameter{
			{
				Name: "value",
				Type: data.PointerType(data.InterfaceType),
			},
		},
		Variadic: true,
		Type:     data.PointerType(t),
		Returns:  []*data.Type{data.ErrorType},
	}, rowsScan)

	t.DefineFunction("Close", &data.Declaration{
		Name:    "Close",
		Type:    data.PointerType(t),
		Returns: []*data.Type{data.ErrorType},
	}, rowsClose)

	t.DefineFunction("Headings", &data.Declaration{
		Name:    "Headings",
		Type:    data.PointerType(t),
		Returns: []*data.Type{data.ArrayType(data.StringType)},
	}, rowsHeadings)

	rowsType = t.SetPackage("db")

	return rowsType
}
