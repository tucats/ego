package db

import (
	"sync"

	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
)

var dbTypeDef *data.Type
var dbTypeDefLock sync.Mutex

var dbRowsTypeDef *data.Type
var dbRowsTypeDefLock sync.Mutex

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

func initDBRowsTypeDef() {
	dbRowsTypeDefLock.Lock()
	defer dbRowsTypeDefLock.Unlock()

	if dbRowsTypeDef == nil {
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
					ParmType: data.PointerType(&data.InterfaceType),
				},
			},
			Variadic:     true,
			ReceiverType: data.PointerType(t),
			ReturnTypes:  []*data.Type{&data.ErrorType},
		}, rowsScan)

		t.DefineFunction("Close", &data.FunctionDeclaration{
			Name:         "Close",
			ReceiverType: data.PointerType(t),
			ReturnTypes:  []*data.Type{&data.ErrorType},
		}, rowsClose)

		t.DefineFunction("Headings", &data.FunctionDeclaration{
			Name:         "Headings",
			ReceiverType: data.PointerType(t),
			ReturnTypes:  []*data.Type{data.ArrayType(&data.StringType)},
		}, rowsHeadings)

		dbRowsTypeDef = t
	}
}
