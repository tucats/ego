// Package scripting implements the tables-service transaction handler.
//
// A "transaction" in this context is an HTTP POST request whose body is a
// JSON array of operation objects (defs.TXOperation). Each operation has an
// opcode that selects the database action to perform, along with a table name,
// optional filter expressions, an optional column list, and a data payload.
// All operations in the array are executed inside a single database transaction:
// if any one of them fails the whole batch is rolled back.
//
// Supported opcodes and the functions that handle them:
//
//	"symbols"  – doSymbols  – load named values into the per-transaction symbol table
//	"select"   – doSelect   – SELECT rows; store the first row of column values as symbols
//	"readrows" – doRows     – SELECT rows; store the full result set as a symbol
//	"insert"   – doInsert   – INSERT a single row
//	"update"   – doUpdate   – UPDATE rows matching a filter
//	"delete"   – doDelete   – DELETE rows matching a filter
//	"drop"     – doDrop     – DROP TABLE
//	"sql"      – doSQL      – execute arbitrary SQL (non-SELECT)
//
// Symbol substitution:
//
// Any string field in an operation (table name, filter, column name, data value,
// error condition) may contain references of the form {{name}}. Before the
// operation is executed, each reference is replaced with the current value of
// that symbol from the per-transaction symbol table. This lets earlier operations
// feed results into later ones without a round-trip to the client.
package scripting

// symbolTable holds named values that are shared across all operations in a
// single transaction. Values are inserted by "symbols" operations and by
// "select"/"readrows" operations that read rows from the database. Other
// operations consume these values through {{name}} substitution.
type symbolTable struct {
	symbols map[string]any
}

const (
	// Opcodes — the allowed values of defs.TXOperation.Opcode.
	symbolsOpcode = "symbols"  // load data items into the symbol table
	insertOpcode  = "insert"   // insert a single row
	deleteOpcode  = "delete"   // delete rows matching a filter
	updateOpcode  = "update"   // update rows matching a filter
	dropOpCode    = "drop"     // drop a table
	selectOpcode  = "select"   // select a single row into symbol variables
	rowsOpcode    = "readrows" // select multiple rows into a result-set symbol
	sqlOpcode     = "sql"      // execute arbitrary SQL

	// SQL verb strings used when building queries.
	selectVerb = "SELECT"
	deleteVerb = "DELETE"

	// sqliteProvider is the provider name string for SQLite databases. Used
	// throughout the scripting package to skip schema qualification, which
	// SQLite does not support via the user-as-schema convention.
	sqliteProvider = "sqlite3"

	// tableMetadataQuery fetches zero rows from a table so we can inspect its
	// column names and types without reading any actual data. The {{schema}} and
	// {{table}} placeholders are filled in by parsing.QueryParameters before the
	// query is sent to the database.
	tableMetadataQuery = `SELECT * FROM {{schema}}.{{table}} WHERE 1=0`

	// tableMetadataSQLiteQuery is the SQLite variant of tableMetadataQuery.
	// SQLite stores tables without a user-schema prefix, so the query uses
	// only the unqualified {{table}} placeholder.
	tableMetadataSQLiteQuery = `SELECT * FROM {{table}} WHERE 1=0`

	// resultSetSymbolName is the key under which doRows stores the full query
	// result in the symbol table. It uses a deliberately unusual string so it
	// cannot clash with user-defined symbol names. After the transaction commits,
	// Handler checks for this key and, if present, returns its value as the
	// HTTP response body instead of a plain row-count.
	resultSetSymbolName = "$$RESULT$$SET$$"
)
