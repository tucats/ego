package db

import (
	"github.com/tucats/ego/data"
)

var DBRowsType *data.Type = data.TypeDefinition("Rows",
	data.StructureType().
		DefineField("client", data.InterfaceType).
		DefineField("rows", data.InterfaceType).
		DefineField("db", data.InterfaceType).
		DefineFunction("Next", &data.Declaration{
			Name:    "Next",
			Type:    data.OwnType,
			Returns: []*data.Type{data.BoolType},
		}, rowsNext).
		DefineFunction("Scan", &data.Declaration{
			Name: "Scan",
			Parameters: []data.Parameter{
				{
					Name: "value",
					Type: data.PointerType(data.InterfaceType),
				},
			},
			Type:     data.OwnType,
			Variadic: true,
			Returns:  []*data.Type{data.ErrorType},
		}, rowsScan).
		DefineFunction("Close", &data.Declaration{
			Name:    "Close",
			Type:    data.OwnType,
			Returns: []*data.Type{data.ErrorType},
		}, rowsClose).
		DefineFunction("Headings", &data.Declaration{
			Name:    "Headings",
			Type:    data.OwnType,
			Returns: []*data.Type{data.ArrayType(data.StringType)},
		}, rowsHeadings),
).SetPackage("db").FixSelfReferences()

var DBClientType *data.Type = data.TypeDefinition("Client",
	data.StructureType().
		DefineField("Client", data.InterfaceType).
		DefineField("asStruct", data.BoolType).
		DefineField("rowCount", data.IntType).
		DefineField("transaction", data.InterfaceType).
		DefineField("constr", data.StringType).
		DefineFunction("Begin", &data.Declaration{
			Name:    "Begin",
			Type:    data.OwnType,
			Returns: []*data.Type{data.ErrorType},
		}, begin).
		DefineFunction("Begin", &data.Declaration{
			Name:    "Begin",
			Type:    data.OwnType,
			Returns: []*data.Type{data.ErrorType},
		}, begin).
		DefineFunction("Commit", &data.Declaration{
			Name:    "Commit",
			Type:    data.OwnType,
			Returns: []*data.Type{data.ErrorType},
		}, commit).
		DefineFunction("Rollback", &data.Declaration{
			Name:    "Rollback",
			Type:    data.OwnType,
			Returns: []*data.Type{data.ErrorType},
		}, rollback).
		DefineFunction("Query", &data.Declaration{
			Name: "Query",
			Type: data.OwnType,
			Parameters: []data.Parameter{
				{
					Name: "sql",
					Type: data.StringType,
				},
				{
					Name: "args",
					Type: data.ArrayType(data.InterfaceType),
				},
			},
			Variadic: true,
			Returns: []*data.Type{
				data.PointerType(DBRowsType),
				data.ErrorType,
			},
		}, query).
		DefineFunction("QueryResult", &data.Declaration{
			Name: "Execute",
			Type: data.OwnType,
			Parameters: []data.Parameter{
				{
					Name: "sql",
					Type: data.StringType,
				},
				{
					Name: "args",
					Type: data.ArrayType(data.InterfaceType),
				},
			},
			Variadic: true,
			Returns: []*data.Type{
				data.ArrayType(data.ArrayType(data.InterfaceType)),
				data.ErrorType,
			},
		}, queryResult).
		DefineFunction("Execute", &data.Declaration{
			Name: "Execute",
			Type: data.OwnType,
			Parameters: []data.Parameter{
				{
					Name: "sql",
					Type: data.StringType,
				},
				{
					Name: "args",
					Type: data.ArrayType(data.InterfaceType),
				},
			},
			Variadic: true,
			Returns: []*data.Type{
				data.IntType,
				data.ErrorType,
			},
		}, execute).
		DefineFunction("Close", &data.Declaration{
			Name:    "Close",
			Type:    data.OwnType,
			Returns: []*data.Type{data.ErrorType},
		}, closeConnection).
		DefineFunction("AsStruct", &data.Declaration{
			Name: "AsStruct",
			Type: data.OwnType,
			Parameters: []data.Parameter{
				{
					Name: "flag",
					Type: data.BoolType,
				},
			},
			Returns: []*data.Type{data.VoidType},
		}, asStructures),
).SetPackage("db").FixSelfReferences()

var DBPackage = data.NewPackageFromMap("db", map[string]any{
	"New": data.Function{
		Declaration: &data.Declaration{
			Name: "New",
			Parameters: []data.Parameter{
				{
					Name: "connection",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{DBClientType},
		},
		Value: newConnection,
	},
	"Client":           DBClientType,
	"Rows":             DBRowsType,
	data.TypeMDKey:     data.PackageType("db"),
	data.ReadonlyMDKey: true,
})

const (
	clientFieldName      = "client"
	constrFieldName      = "Constr"
	dbFieldName          = "db"
	rowCountFieldName    = "Rowcount"
	rowsFieldName        = "rows"
	asStructFieldName    = "asStruct"
	transactionFieldName = "transaction"
)
