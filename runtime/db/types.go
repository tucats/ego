package db

import (
	"sync"

	"github.com/tucats/ego/compiler"
	"github.com/tucats/ego/data"
)

var rowsType *data.Type
var clientType *data.Type
var typeDefLock sync.Mutex

func initClientTypeDef() {
	typeDefLock.Lock()
	defer typeDefLock.Unlock()

	if clientType == nil {
		initRowsTypeDef()

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

		clientType = t
	}
}

func initRowsTypeDef() {
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

	rowsType = t
}
