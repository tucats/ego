// Package db provides the Ego scripting language's database access layer,
// mirroring the standard "database/sql" package in conventional Go. It
// defines two Ego-visible types (db.Client and db.Rows) and exposes a single
// package-level constructor, db.New(), that opens a connection.
//
// All functions in this package share the same signature:
//
//	func(s *symbols.SymbolTable, args data.List) (any, error)
//
// The symbol table (s) always carries a defs.ThisVariable ("__this") which
// is a *data.Struct representing the receiver — either a Client struct or a
// Rows cursor struct.
//
// Field name constants (defined at the bottom of this file) are the stable
// identifiers used to read and write named fields on those structs from Go
// code. Using constants prevents typos and makes refactoring safe.
package sql

import (
	"github.com/tucats/ego/data"
)

// RowsType is the Ego type definition for a db.Rows cursor. Instances are
// created by db.Client.Query() and carry three internal fields:
//
//   - "rows"   — the *sql.Rows handle from the underlying database driver
//   - "client" — the *sql.DB handle (kept so the cursor can reach the
//     connection pool if needed)
//   - "db"     — a back-reference to the owning db.Client *data.Struct,
//     used by rowsScan to read the asStruct flag
//
// The four methods (Next, Scan, Close, Headings) delegate to Go functions
// in rows.go.
var RowsType *data.Type = data.TypeDefinition("Rows",
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
).SetPackage("sql").FixSelfReferences()

// Database is the Ego type definition for a db.Client connection handle.
// Instances are created by db.New() (newConnection in db.go) and expose the
// database connection along with its current state:
//
//   - "Client"      — the *sql.DB connection pool handle
//   - "asStruct"    — bool; when true Query/QueryResult return rows as
//     *data.Struct values keyed by column name, otherwise
//     each row is a *data.Array of column values in order
//   - "rowCount"    — int; number of rows affected by the last Execute call
//   - "transaction" — *sql.Tx or nil; non-nil while a transaction is active
//   - "constr"      — string; the (redacted) connection URL for diagnostics
//
// NOTE: "Begin" is registered twice below (lines 48-57 and 53-57). Both
// registrations point to the same begin() function, so behavior is correct,
// but the second definition is dead code — it silently overwrites the first
// entry in the method table. A future cleanup should remove the duplicate.
var Database *data.Type = data.TypeDefinition("Database",
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
				data.PointerType(RowsType),
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

// DBPackage is the Ego package object that the import system installs when
// Ego code writes `import "db"`. It exports:
//   - db.New(connection string) — opens a new connection and returns a Client
//   - db.Client               — the Client type (for type assertions / docs)
//   - db.Rows                 — the Rows type (for type assertions / docs)
var SqlPackage = data.NewPackageFromMap("sql", map[string]any{
	"Open": data.Function{
		Declaration: &data.Declaration{
			Name: "Open",
			Parameters: []data.Parameter{
				{
					Name: "driver",
					Type: data.StringType,
				},
				{
					Name: "connection",
					Type: data.StringType,
				},
			},
			Returns: []*data.Type{Database, data.ErrorType},
		},
		Value: openDatabase,
	},
	"Database":         Database,
	"Rows":             RowsType,
	data.TypeMDKey:     data.PackageType("sql"),
	data.ReadonlyMDKey: true,
})

// Field name constants for the two Ego struct types managed by this package.
// Using constants rather than raw string literals prevents typos, keeps all
// names in one place, and makes the compiler flag any mismatches.
//
// Client struct fields:
//
//	clientFieldName      — holds *sql.DB; nil means the connection was closed
//	constrFieldName      — the (possibly redacted) connection URL string
//	rowCountFieldName    — rows affected by the last Execute call
//	asStructFieldName    — bool mode selector for Query/QueryResult results
//	transactionFieldName — holds *sql.Tx; nil when no transaction is active
//
// Rows struct fields:
//
//	rowsFieldName — holds *sql.Rows; nil after the cursor is closed
//	dbFieldName   — back-reference to the Client struct (for asStruct flag)
//	clientFieldName is reused on the Rows struct to hold the *sql.DB handle
const (
	clientFieldName      = "client"
	constrFieldName      = "Constr"
	dbFieldName          = "db"
	rowCountFieldName    = "Rowcount"
	rowsFieldName        = "rows"
	asStructFieldName    = "asStruct"
	transactionFieldName = "transaction"
)
