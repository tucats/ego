package runtime

import (
	"sync"

	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
)

// db.Client type specification.
const dbTypeSpec = `
type db.Client struct {
	client 		interface{},
	asStruct 	bool,
	rowCount 	int,
	transaction	interface{},
	constr 		string,
}`

var dbTypeDef *data.Type
var dbTypeDefLock sync.Mutex

func initDBTypeDef() {
	dbTypeDefLock.Lock()
	defer dbTypeDefLock.Unlock()

	if dbTypeDef == nil {
		initDBRowsTypeDef()

		t, _ := compiler.CompileTypeSpec(dbTypeSpec)

		t.DefineFunction("Begin", &data.FunctionDeclaration{
			Name:         "Begin",
			ReceiverType: data.PointerType(t),
			ReturnTypes:  []*data.Type{&data.ErrorType},
		}, DBBegin)

		t.DefineFunction("Commit", &data.FunctionDeclaration{
			Name:         "Commit",
			ReceiverType: data.PointerType(t),
			ReturnTypes:  []*data.Type{&data.ErrorType},
		}, DBCommit)

		t.DefineFunction("Rollback", &data.FunctionDeclaration{
			Name:         "Rollback",
			ReceiverType: data.PointerType(t),
			ReturnTypes:  []*data.Type{&data.ErrorType},
		}, DBRollback)

		t.DefineFunction("Query", &data.FunctionDeclaration{
			Name:         "Query",
			ReceiverType: data.PointerType(t),
			Parameters: []data.FunctionParameter{
				{
					Name:     "sql",
					ParmType: &data.StringType,
				},
			},
			ReturnTypes: []*data.Type{
				data.PointerType(dbRowsTypeDef),
				&data.ErrorType,
			},
		}, DBQueryRows)

		t.DefineFunction("QueryResult", &data.FunctionDeclaration{
			Name:         "Execute",
			ReceiverType: data.PointerType(t),
			Parameters: []data.FunctionParameter{
				{
					Name:     "sql",
					ParmType: &data.StringType,
				},
			},
			ReturnTypes: []*data.Type{
				data.ArrayType(data.ArrayType(&data.InterfaceType)),
				&data.ErrorType,
			},
		}, DBQuery)

		t.DefineFunction("Execute", &data.FunctionDeclaration{
			Name:         "Execute",
			ReceiverType: data.PointerType(t),
			Parameters: []data.FunctionParameter{
				{
					Name:     "sql",
					ParmType: &data.StringType,
				},
			},
			ReturnTypes: []*data.Type{
				&data.IntType,
				&data.ErrorType,
			},
		}, DBExecute)

		t.DefineFunction("Close", &data.FunctionDeclaration{
			Name:         "Close",
			ReceiverType: data.PointerType(t),
			ReturnTypes:  []*data.Type{&data.ErrorType},
		}, DBClose)

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
		}, DataBaseAsStruct)

		dbTypeDef = t
	}
}
