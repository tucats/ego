// Package db provides tests for the runtime/db package, which implements
// database access for the Ego scripting language. Tests use SQLite3 via
// temporary files so they require no external database infrastructure.
//
// All database function signatures follow the pattern:
//
//	func(s *symbols.SymbolTable, args data.List) (any, error)
//
// The symbol table carries defs.ThisVariable ("__this") which is a *data.Struct
// representing either a DB Client or a Rows cursor. Tests construct the
// appropriate symbol table and call the functions directly.
package sql

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/symbols"
)

const favoriteCousin = "Alice"

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// makeTestDB creates a temporary directory, returns the path to a test.db file
// inside it, and a cleanup function that removes the directory when done.
func makeTestDB(t *testing.T) (dbPath string, cleanup func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "ego-db-test-*")
	if err != nil {
		t.Fatalf("makeTestDB: failed to create temp dir: %v", err)
	}

	dbPath = filepath.Join(dir, "test.db")
	cleanup = func() { os.RemoveAll(dir) }

	return dbPath, cleanup
}

// makeClientST calls newConnection with a sqlite3:// URL, stores the resulting
// *data.Struct as __this in a new symbol table, and returns both. The test is
// fatally aborted if newConnection returns an error.
func makeClientST(t *testing.T, dbPath string) (*symbols.SymbolTable, *data.Struct) {
	t.Helper()

	s := symbols.NewSymbolTable("testing")
	connStr := dbPath
	args := data.NewList("sqlite3", connStr)

	result, err := openDatabase(s, args)
	if err != nil {
		t.Fatalf("makeClientST: newConnection failed: %v", err)
	}

	tuple, ok := result.(data.List)
	if !ok {
		t.Fatalf("makeClientST: expected data.List, got %T", result)
	}

	clientStruct, ok := tuple.Get(0).(*data.Struct)
	if !ok {
		t.Fatalf("makeClientST: expected *data.Struct, got %T", result)
	}

	s.SetAlways(defs.ThisVariable, clientStruct)

	return s, clientStruct
}

// makeRowsST stores a Rows *data.Struct as __this in a new symbol table.
func makeRowsST(rowsStruct *data.Struct) *symbols.SymbolTable {
	s := symbols.NewSymbolTable("testing-rows")
	s.SetAlways(defs.ThisVariable, rowsStruct)

	return s
}

// setupSchema executes CREATE TABLE and three INSERTs on the given symbol
// table. It uses the execute() function directly.
func setupSchema(t *testing.T, s *symbols.SymbolTable) {
	t.Helper()

	sqlStatements := []string{
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)`,
		`INSERT INTO users (id, name, age) VALUES (1, 'Alice', 30)`,
		`INSERT INTO users (id, name, age) VALUES (2, 'Bob', 25)`,
		`INSERT INTO users (id, name, age) VALUES (3, 'Charlie', 35)`,
	}

	for _, stmt := range sqlStatements {
		result, err := execute(s, data.NewList(stmt))
		if err != nil {
			t.Fatalf("setupSchema: execute(%q) go-error: %v", stmt, err)
		}

		if lErr := listErr(result); lErr != nil {
			t.Fatalf("setupSchema: execute(%q) failed: %v", stmt, lErr)
		}
	}
}

// assertErrNil fatally fails the test if err is non-nil.
func assertErrNil(t *testing.T, label string, err error) {
	t.Helper()

	if err != nil && !errors.Nil(err) {
		t.Fatalf("%s: unexpected error: %v", label, err)
	}
}

// listErr extracts the last element of a data.List result as an error.
// Runtime functions that use the data.NewList(...) pattern embed errors
// in the list rather than returning them as a Go error; callers must use
// this helper to check for failures.
func listErr(result any) error {
	list, ok := result.(data.List)
	if !ok || list.Len() == 0 {
		return nil
	}

	last := list.Get(list.Len() - 1)
	if last == nil {
		return nil
	}

	if e, ok := last.(error); ok {
		return e
	}

	return nil
}

// ---------------------------------------------------------------------------
// newConnection tests
// ---------------------------------------------------------------------------

func TestNewConnection_ValidSQLite3FilePath(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s := symbols.NewSymbolTable("testing")
	args := data.NewList("sqlite3", dbPath)

	result, err := openDatabase(s, args)

	assertErrNil(t, "newConnection", err)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	tuple, ok := result.(data.List)
	if !ok {
		t.Fatalf("expected data.List, got %T", result)
	}

	clientStruct, ok := tuple.Get(0).(*data.Struct)
	if !ok {
		t.Fatalf("expected *data.Struct, got %T", result)
	}

	// The client field must be populated.
	clientVal := clientStruct.GetAlways(clientFieldName)
	if clientVal == nil {
		t.Fatal("client field must not be nil after newConnection")
	}
}

func TestNewConnection_InvalidURL(t *testing.T) {
	s := symbols.NewSymbolTable("testing")

	// A URL with an invalid character sequence that url.Parse will reject.
	// The Go url.Parse is actually quite lenient, so use a scheme-less
	// string that leads to sql.Open failing for an unsupported driver.
	args := data.NewList("://this-is-not-valid")

	_, err := openDatabase(s, args)

	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestNewConnection_BlockedCredentialDB(t *testing.T) {
	// Attempting to open the credentials database must be rejected.
	s := symbols.NewSymbolTable("testing")
	args := data.NewList("sqlite3", "/some/path/ego-system.db")

	_, err := openDatabase(s, args)

	if err == nil {
		t.Fatal("expected ErrNoPrivilegeForOperation, got nil")
	}

	if !errors.Equal(err, errors.ErrNoPrivilegeForOperation) {
		t.Fatalf("expected ErrNoPrivilegeForOperation, got: %v", err)
	}
}

func TestNewConnection_ConnectionStringStored(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s := symbols.NewSymbolTable("testing")
	args := data.NewList("sqlite3", dbPath)

	result, err := openDatabase(s, args)
	assertErrNil(t, "newConnection", err)

	tuple := result.(data.List)
	clientStruct := tuple.Get(0).(*data.Struct)
	constr := data.String(clientStruct.GetAlways(constrFieldName))

	if constr == "" {
		t.Fatal("expected non-empty connection string stored on struct")
	}
}

// ---------------------------------------------------------------------------
// asStructures tests
// ---------------------------------------------------------------------------

func TestAsStructures_SetTrue(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, clientStruct := makeClientST(t, dbPath)
	args := data.NewList(true)

	_, err := asStructures(s, args)
	assertErrNil(t, "asStructures(true)", err)

	if !data.BoolOrFalse(clientStruct.GetAlways(asStructFieldName)) {
		t.Fatal("expected asStruct to be true")
	}
}

func TestAsStructures_SetFalse(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, clientStruct := makeClientST(t, dbPath)

	// First set to true, then back to false.
	_, _ = asStructures(s, data.NewList(true))
	_, err := asStructures(s, data.NewList(false))
	assertErrNil(t, "asStructures(false)", err)

	if data.BoolOrFalse(clientStruct.GetAlways(asStructFieldName)) {
		t.Fatal("expected asStruct to be false")
	}
}

// ---------------------------------------------------------------------------
// closeConnection tests
// ---------------------------------------------------------------------------

func TestCloseConnection_Normal(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, clientStruct := makeClientST(t, dbPath)

	result, err := closeConnection(s, data.NewList())
	assertErrNil(t, "closeConnection", err)

	if lErr := listErr(result); lErr != nil {
		t.Fatalf("expected nil error from closeConnection, got %v", lErr)
	}

	// After close the client field must be nil.
	if clientStruct.GetAlways(clientFieldName) != nil {
		t.Fatal("expected clientFieldName to be nil after close")
	}
}

func TestCloseConnection_DoubleClose_ReturnsError(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)

	// First close succeeds.
	_, err := closeConnection(s, data.NewList())
	assertErrNil(t, "first closeConnection", err)

	// Second close must fail. After the first close, the struct's clientFieldName
	// is set to nil. The client() helper only recognizes the *sql.DB when the
	// field is non-nil, so when the field is nil it falls through and returns
	// ErrNoFunctionReceiver (not ErrDatabaseClientClosed, which would require the
	// field to hold a typed nil *sql.DB pointer — a subtle Go nil-interface quirk).
	result2, _ := closeConnection(s, data.NewList())

	lErr := listErr(result2)
	if lErr == nil {
		t.Fatal("expected error on double close, got nil")
	}

	if !errors.Equal(lErr, errors.ErrNoFunctionReceiver) {
		t.Fatalf("expected ErrNoFunctionReceiver after double close, got: %v", lErr)
	}
}

// ---------------------------------------------------------------------------
// execute tests
// ---------------------------------------------------------------------------

func TestExecute_CreateTable(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)
	args := data.NewList(`CREATE TABLE IF NOT EXISTS t1 (id INTEGER PRIMARY KEY, val TEXT)`)

	result, err := execute(s, args)
	assertErrNil(t, "execute CREATE TABLE", err)

	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("expected data.List, got %T", result)
	}

	rowCount, cvtErr := data.Int(list.Get(0))
	if cvtErr != nil {
		t.Fatalf("could not convert row count: %v", cvtErr)
	}

	// DDL typically reports 0 rows affected.
	if rowCount != 0 {
		t.Fatalf("expected 0 rows affected for CREATE TABLE, got %d", rowCount)
	}
}

func TestExecute_Insert(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)
	_, _ = execute(s, data.NewList(`CREATE TABLE IF NOT EXISTS t1 (id INTEGER PRIMARY KEY, val TEXT)`))

	result, err := execute(s, data.NewList(`INSERT INTO t1 (id, val) VALUES (1, 'hello')`))
	assertErrNil(t, "execute INSERT", err)

	list := result.(data.List)
	rowCount, _ := data.Int(list.Get(0))

	if rowCount != 1 {
		t.Fatalf("expected 1 row affected, got %d", rowCount)
	}
}

func TestExecute_MultiRowDelete(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)
	setupSchema(t, s)

	result, err := execute(s, data.NewList(`DELETE FROM users WHERE age > 0`))
	assertErrNil(t, "execute DELETE", err)

	list := result.(data.List)
	rowCount, _ := data.Int(list.Get(0))

	if rowCount != 3 {
		t.Fatalf("expected 3 rows deleted, got %d", rowCount)
	}
}

func TestExecute_NoArgs_ReturnsError(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)

	result, _ := execute(s, data.NewList())

	lErr := listErr(result)
	if lErr == nil {
		t.Fatal("expected ErrArgumentCount for zero args, got nil")
	}

	if !errors.Equal(lErr, errors.ErrArgumentCount) {
		t.Fatalf("expected ErrArgumentCount, got: %v", lErr)
	}
}

func TestExecute_InvalidSQL_ReturnsError(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)

	result, _ := execute(s, data.NewList(`NOT VALID SQL !!!`))

	if listErr(result) == nil {
		t.Fatal("expected error for invalid SQL, got nil")
	}
}

// ---------------------------------------------------------------------------
// query tests
// ---------------------------------------------------------------------------

func TestQuery_ValidSelect(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)
	setupSchema(t, s)

	result, err := query(s, data.NewList(`SELECT id, name, age FROM users ORDER BY id`))
	assertErrNil(t, "query", err)

	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("expected data.List, got %T", result)
	}

	rowsStruct, ok := list.Get(0).(*data.Struct)
	if !ok {
		t.Fatalf("expected *data.Struct for rows at index 0, got %T", list.Get(0))
	}

	// Verify the rows object contains the actual sql.Rows.
	rowsVal := rowsStruct.GetAlways(rowsFieldName)
	if rowsVal == nil {
		t.Fatal("rows field must not be nil")
	}

	// Clean up by closing the rows.
	rowsST := makeRowsST(rowsStruct)
	_, _ = rowsClose(rowsST, data.NewList())
}

func TestQuery_NoArgs_ReturnsError(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)

	result, _ := query(s, data.NewList())

	lErr := listErr(result)
	if lErr == nil {
		t.Fatal("expected ErrArgumentCount, got nil")
	}

	if !errors.Equal(lErr, errors.ErrArgumentCount) {
		t.Fatalf("expected ErrArgumentCount, got: %v", lErr)
	}
}

func TestQuery_InvalidSQL_ReturnsError(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)

	result, _ := query(s, data.NewList(`SELECT * FROM nonexistent_table_xyz`))

	if listErr(result) == nil {
		t.Fatal("expected error for invalid SQL, got nil")
	}
}

// ---------------------------------------------------------------------------
// queryResult tests (array mode, asStruct=false)
// ---------------------------------------------------------------------------

func TestQueryResult_ArrayMode_ReturnsRows(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, clientStruct := makeClientST(t, dbPath)
	setupSchema(t, s)

	// Ensure asStruct is false (default).
	clientStruct.SetAlways(asStructFieldName, false)

	result, err := queryResult(s, data.NewList(`SELECT id, name, age FROM users ORDER BY id`))
	assertErrNil(t, "queryResult", err)

	list, ok := result.(data.List)
	if !ok {
		t.Fatalf("expected data.List, got %T", result)
	}

	arr, ok := list.Get(0).(*data.Array)
	if !ok {
		t.Fatalf("expected *data.Array at index 0, got %T", list.Get(0))
	}

	if arr.Len() != 3 {
		t.Fatalf("expected 3 rows, got %d", arr.Len())
	}
}

func TestQueryResult_ArrayMode_RowValuesCorrect(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, clientStruct := makeClientST(t, dbPath)
	setupSchema(t, s)
	clientStruct.SetAlways(asStructFieldName, false)

	result, err := queryResult(s, data.NewList(`SELECT id, name, age FROM users WHERE id=1`))
	assertErrNil(t, "queryResult single row", err)

	list := result.(data.List)
	arr := list.Get(0).(*data.Array)

	if arr.Len() != 1 {
		t.Fatalf("expected 1 row, got %d", arr.Len())
	}

	firstRow, getErr := arr.Get(0)
	if getErr != nil {
		t.Fatalf("arr.Get(0): %v", getErr)
	}

	rowArr, ok := firstRow.(*data.Array)
	if !ok {
		t.Fatalf("expected inner *data.Array, got %T", firstRow)
	}

	nameVal, _ := rowArr.Get(1)
	if data.String(nameVal) != favoriteCousin {
		t.Fatalf("expected name=Alice, got %v", nameVal)
	}

	ageVal, _ := rowArr.Get(2)
	ageInt, _ := data.Int(ageVal)

	if ageInt != 30 {
		t.Fatalf("expected age=30, got %v", ageVal)
	}
}

func TestQueryResult_NoArgs_ReturnsError(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)

	result, _ := queryResult(s, data.NewList())

	lErr := listErr(result)
	if lErr == nil {
		t.Fatal("expected ErrArgumentCount, got nil")
	}

	if !errors.Equal(lErr, errors.ErrArgumentCount) {
		t.Fatalf("expected ErrArgumentCount, got: %v", lErr)
	}
}

// ---------------------------------------------------------------------------
// queryResult tests (struct mode, asStruct=true)
// ---------------------------------------------------------------------------

func TestQueryResult_StructMode_ReturnsStructs(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, clientStruct := makeClientST(t, dbPath)
	setupSchema(t, s)
	clientStruct.SetAlways(asStructFieldName, true)

	result, err := queryResult(s, data.NewList(`SELECT id, name, age FROM users ORDER BY id`))
	assertErrNil(t, "queryResult struct mode", err)

	list := result.(data.List)

	arr, ok := list.Get(0).(*data.Array)
	if !ok {
		t.Fatalf("expected *data.Array, got %T", list.Get(0))
	}

	if arr.Len() != 3 {
		t.Fatalf("expected 3 rows, got %d", arr.Len())
	}

	firstElem, _ := arr.Get(0)

	rowStruct, ok := firstElem.(*data.Struct)
	if !ok {
		t.Fatalf("expected *data.Struct for each row, got %T", firstElem)
	}

	// The struct must have a "name" field (column name).
	nameVal := rowStruct.GetAlways("name")
	if data.String(nameVal) != favoriteCousin {
		t.Fatalf("expected name=Alice, got %v", nameVal)
	}
}

// ---------------------------------------------------------------------------
// Row navigation tests
// ---------------------------------------------------------------------------

func TestRowsHeadings_ReturnsColumnNames(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)
	setupSchema(t, s)

	queryRes, err := query(s, data.NewList(`SELECT id, name, age FROM users`))
	assertErrNil(t, "query for headings test", err)

	list := queryRes.(data.List)
	rowsStruct := list.Get(0).(*data.Struct)
	rowsST := makeRowsST(rowsStruct)

	headResult, err := rowsHeadings(rowsST, data.NewList())
	assertErrNil(t, "rowsHeadings", err)

	colArr, ok := headResult.(*data.Array)
	if !ok {
		t.Fatalf("expected *data.Array from rowsHeadings, got %T", headResult)
	}

	if colArr.Len() != 3 {
		t.Fatalf("expected 3 columns, got %d", colArr.Len())
	}

	expected := []string{"id", "name", "age"}
	for i, exp := range expected {
		v, _ := colArr.Get(i)
		if data.String(v) != exp {
			t.Fatalf("column[%d]: expected %q, got %q", i, exp, data.String(v))
		}
	}

	// Close rows to free resources.
	_, _ = rowsClose(rowsST, data.NewList())
}

func TestRowsNext_ReturnsTrueForFirstRow(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)
	setupSchema(t, s)

	queryRes, err := query(s, data.NewList(`SELECT id, name, age FROM users ORDER BY id`))
	assertErrNil(t, "query", err)

	list := queryRes.(data.List)
	rowsStruct := list.Get(0).(*data.Struct)
	rowsST := makeRowsST(rowsStruct)

	hasNext, err := rowsNext(rowsST, data.NewList())
	assertErrNil(t, "rowsNext first", err)

	if hasNext != true {
		t.Fatal("expected rowsNext to return true for first row")
	}

	// Close rows to free resources.
	_, _ = rowsClose(rowsST, data.NewList())
}

func TestRowsNext_EventuallyReturnsFalse(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)
	setupSchema(t, s)

	queryRes, err := query(s, data.NewList(`SELECT id FROM users ORDER BY id`))
	assertErrNil(t, "query", err)

	list := queryRes.(data.List)
	rowsStruct := list.Get(0).(*data.Struct)
	rowsST := makeRowsST(rowsStruct)

	// Drain all 3 rows.
	count := 0

	for {
		hasNext, _ := rowsNext(rowsST, data.NewList())

		if hasNext != true {
			break
		}

		count++

		// Safety valve to avoid infinite loop.
		if count > 100 {
			t.Fatal("rowsNext never returned false")
		}
	}

	if count != 3 {
		t.Fatalf("expected 3 rows, iterated %d", count)
	}

	_, _ = rowsClose(rowsST, data.NewList())
}

func TestRowsScan_NoArgs_ReturnsArrayOfValues(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, clientStruct := makeClientST(t, dbPath)
	setupSchema(t, s)
	clientStruct.SetAlways(asStructFieldName, false)

	queryRes, err := query(s, data.NewList(`SELECT id, name, age FROM users WHERE id=1`))
	assertErrNil(t, "query", err)

	list := queryRes.(data.List)
	rowsStruct := list.Get(0).(*data.Struct)
	// The rows struct needs a back-reference to the client struct so that
	// rowsScan can read the asStruct flag.
	rowsStruct.SetAlways(dbFieldName, clientStruct)
	rowsST := makeRowsST(rowsStruct)

	// Advance to first row.
	_, _ = rowsNext(rowsST, data.NewList())

	scanResult, err := rowsScan(rowsST, data.NewList())
	assertErrNil(t, "rowsScan", err)

	scanList, ok := scanResult.(data.List)
	if !ok {
		t.Fatalf("expected data.List from rowsScan, got %T", scanResult)
	}

	rowArr, ok := scanList.Get(0).(*data.Array)
	if !ok {
		t.Fatalf("expected *data.Array in scan list, got %T", scanList.Get(0))
	}

	nameVal, _ := rowArr.Get(1)
	if data.String(nameVal) != favoriteCousin {
		t.Fatalf("expected Alice, got %v", nameVal)
	}

	_, _ = rowsClose(rowsST, data.NewList())
}

func TestRowsScan_WithPointerArgs_FillsValues(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, clientStruct := makeClientST(t, dbPath)
	setupSchema(t, s)
	clientStruct.SetAlways(asStructFieldName, false)

	queryRes, err := query(s, data.NewList(`SELECT id, name, age FROM users WHERE id=2`))
	assertErrNil(t, "query", err)

	list := queryRes.(data.List)
	rowsStruct := list.Get(0).(*data.Struct)
	rowsStruct.SetAlways(dbFieldName, clientStruct)
	rowsST := makeRowsST(rowsStruct)

	_, _ = rowsNext(rowsST, data.NewList())

	var id, name, age any
	scanResult, err := rowsScan(rowsST, data.NewList(&id, &name, &age))
	assertErrNil(t, "rowsScan with pointers", err)

	scanList := scanResult.(data.List)
	// With pointer args the first element should be nil (success indicator).
	if scanList.Get(0) != nil {
		t.Fatalf("expected nil first element when using pointer args, got %v", scanList.Get(0))
	}

	if data.String(name) != "Bob" {
		t.Fatalf("expected name=Bob, got %v", name)
	}

	_, _ = rowsClose(rowsST, data.NewList())
}

func TestRowsClose_SetsRowsNil(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)
	setupSchema(t, s)

	queryRes, err := query(s, data.NewList(`SELECT id FROM users`))
	assertErrNil(t, "query", err)

	list := queryRes.(data.List)
	rowsStruct := list.Get(0).(*data.Struct)
	rowsST := makeRowsST(rowsStruct)

	_, err = rowsClose(rowsST, data.NewList())
	assertErrNil(t, "rowsClose", err)

	// After close the rows field must be nil.
	rowsVal := rowsStruct.GetAlways(rowsFieldName)
	if rowsVal != nil {
		t.Fatalf("expected rows field to be nil after close, got %v", rowsVal)
	}
}

func TestRowsClose_TooManyArgs_ReturnsError(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)
	setupSchema(t, s)

	queryRes, _ := query(s, data.NewList(`SELECT id FROM users`))
	list := queryRes.(data.List)
	rowsStruct := list.Get(0).(*data.Struct)
	rowsST := makeRowsST(rowsStruct)

	result, _ := rowsClose(rowsST, data.NewList("extra"))

	lErr := listErr(result)
	if lErr == nil {
		t.Fatal("expected ErrArgumentCount for extra arg, got nil")
	}

	if !errors.Equal(lErr, errors.ErrArgumentCount) {
		t.Fatalf("expected ErrArgumentCount, got %v", lErr)
	}

	// Close properly to avoid resource leak.
	_, _ = rowsClose(rowsST, data.NewList())
}

// ---------------------------------------------------------------------------
// Transaction tests
// ---------------------------------------------------------------------------

func TestBegin_StartsTransaction(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, clientStruct := makeClientST(t, dbPath)

	_, err := begin(s, data.NewList())
	assertErrNil(t, "begin", err)

	txVal := clientStruct.GetAlways(transactionFieldName)
	if txVal == nil {
		t.Fatal("expected non-nil transaction after begin")
	}

	// Clean up: rollback so we don't leave an open transaction.
	_, _ = rollback(s, data.NewList())
}

func TestBegin_Twice_ReturnsError(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)

	_, err := begin(s, data.NewList())
	assertErrNil(t, "first begin", err)

	result2, _ := begin(s, data.NewList())

	lErr := listErr(result2)
	if lErr == nil {
		t.Fatal("expected ErrTransactionAlreadyActive, got nil")
	}

	if !errors.Equal(lErr, errors.ErrTransactionAlreadyActive) {
		t.Fatalf("expected ErrTransactionAlreadyActive, got: %v", lErr)
	}

	_, _ = rollback(s, data.NewList())
}

func TestBegin_WithExtraArgs_ReturnsError(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)

	result, _ := begin(s, data.NewList("unexpected"))

	lErr := listErr(result)
	if lErr == nil {
		t.Fatal("expected ErrArgumentCount, got nil")
	}

	if !errors.Equal(lErr, errors.ErrArgumentCount) {
		t.Fatalf("expected ErrArgumentCount, got: %v", lErr)
	}
}

func TestCommit_AfterBegin_NoError(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, clientStruct := makeClientST(t, dbPath)
	setupSchema(t, s)

	_, err := begin(s, data.NewList())
	assertErrNil(t, "begin", err)

	_, err = execute(s, data.NewList(`INSERT INTO users (id, name, age) VALUES (99, 'Dave', 40)`))
	assertErrNil(t, "insert inside tx", err)

	_, err = commit(s, data.NewList())
	assertErrNil(t, "commit", err)

	// After commit the transaction field must be nil.
	txVal := clientStruct.GetAlways(transactionFieldName)
	if txVal != nil {
		t.Fatalf("expected transaction to be nil after commit, got %v", txVal)
	}

	// The row must be visible after commit.
	res, err := queryResult(s, data.NewList(`SELECT id FROM users WHERE id=99`))
	assertErrNil(t, "query after commit", err)

	resList := res.(data.List)
	arr := resList.Get(0).(*data.Array)

	if arr.Len() != 1 {
		t.Fatalf("expected 1 row after commit, got %d", arr.Len())
	}
}

func TestRollback_AfterBegin_NoError(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, clientStruct := makeClientST(t, dbPath)
	setupSchema(t, s)

	_, err := begin(s, data.NewList())
	assertErrNil(t, "begin", err)

	_, err = execute(s, data.NewList(`INSERT INTO users (id, name, age) VALUES (88, 'Eve', 22)`))
	assertErrNil(t, "insert inside tx", err)

	_, err = rollback(s, data.NewList())
	assertErrNil(t, "rollback", err)

	// After rollback the transaction field must be nil.
	txVal := clientStruct.GetAlways(transactionFieldName)
	if txVal != nil {
		t.Fatalf("expected transaction to be nil after rollback, got %v", txVal)
	}

	// The row must NOT be visible after rollback.
	res, err := queryResult(s, data.NewList(`SELECT id FROM users WHERE id=88`))
	assertErrNil(t, "query after rollback", err)

	resList := res.(data.List)
	arr := resList.Get(0).(*data.Array)

	if arr.Len() != 0 {
		t.Fatalf("expected 0 rows after rollback, got %d", arr.Len())
	}
}

func TestCommit_WithoutBegin_ReturnsError(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)

	result, _ := commit(s, data.NewList())

	lErr := listErr(result)
	if lErr == nil {
		t.Fatal("expected ErrNoTransactionActive, got nil")
	}

	if !errors.Equal(lErr, errors.ErrNoTransactionActive) {
		t.Fatalf("expected ErrNoTransactionActive, got: %v", lErr)
	}
}

func TestRollback_WithoutBegin_ReturnsError(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)

	result, _ := rollback(s, data.NewList())

	lErr := listErr(result)
	if lErr == nil {
		t.Fatal("expected ErrNoTransactionActive, got nil")
	}

	if !errors.Equal(lErr, errors.ErrNoTransactionActive) {
		t.Fatalf("expected ErrNoTransactionActive, got: %v", lErr)
	}
}

func TestCommit_WithExtraArgs_ReturnsError(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)

	result, _ := commit(s, data.NewList("unexpected"))

	lErr := listErr(result)
	if lErr == nil {
		t.Fatal("expected ErrArgumentCount, got nil")
	}

	if !errors.Equal(lErr, errors.ErrArgumentCount) {
		t.Fatalf("expected ErrArgumentCount, got: %v", lErr)
	}
}

func TestRollback_WithExtraArgs_ReturnsError(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)

	result, _ := rollback(s, data.NewList("unexpected"))

	lErr := listErr(result)
	if lErr == nil {
		t.Fatal("expected ErrArgumentCount, got nil")
	}

	if !errors.Equal(lErr, errors.ErrArgumentCount) {
		t.Fatalf("expected ErrArgumentCount, got: %v", lErr)
	}
}

func TestExecuteInsideTransaction_Rollback_RowAbsent(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)
	setupSchema(t, s)

	_, err := begin(s, data.NewList())
	assertErrNil(t, "begin", err)

	_, err = execute(s, data.NewList(`INSERT INTO users (id, name, age) VALUES (77, 'Frank', 50)`))
	assertErrNil(t, "insert in tx", err)

	_, err = rollback(s, data.NewList())
	assertErrNil(t, "rollback", err)

	res, err := queryResult(s, data.NewList(`SELECT id FROM users WHERE id=77`))
	assertErrNil(t, "query after rollback", err)

	resList := res.(data.List)
	arr := resList.Get(0).(*data.Array)

	if arr.Len() != 0 {
		t.Fatalf("expected 0 rows after rollback, got %d", arr.Len())
	}
}

func TestExecuteInsideTransaction_Commit_RowPresent(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)
	setupSchema(t, s)

	_, err := begin(s, data.NewList())
	assertErrNil(t, "begin", err)

	_, err = execute(s, data.NewList(`INSERT INTO users (id, name, age) VALUES (66, 'Grace', 28)`))
	assertErrNil(t, "insert in tx", err)

	_, err = commit(s, data.NewList())
	assertErrNil(t, "commit", err)

	res, err := queryResult(s, data.NewList(`SELECT id FROM users WHERE id=66`))
	assertErrNil(t, "query after commit", err)

	resList := res.(data.List)
	arr := resList.Get(0).(*data.Array)

	if arr.Len() != 1 {
		t.Fatalf("expected 1 row after commit, got %d", arr.Len())
	}
}

// ---------------------------------------------------------------------------
// PROBLEM AREA DOCUMENTATION TESTS
//
// These tests verify (or document the absence of protection against) known
// bugs found during code review.
// ---------------------------------------------------------------------------

// Previously, types.go defined Begin twice on DBClientType (the second call
// silently overwrote the first). The duplicate has been removed. This test
// verifies that Begin still works correctly after the cleanup.
func TestBug_DuplicateBeginDefinition_StillWorks(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)

	_, err := begin(s, data.NewList())
	assertErrNil(t, "begin", err)

	_, _ = rollback(s, data.NewList())
}

// Previously, rowsClose/rowsNext/rowsScan/rowsHeadings had no nil check before
// casting the rows field, causing a panic when called after Close(). Nil guards
// have been added. rowsNext returns false (not an error) on a closed cursor;
// rowsHeadings returns ErrDatabaseClientClosed as a Go error; rowsClose embeds
// ErrDatabaseClientClosed in a data.List.
func TestBug_NilRowsPanic_AfterClose(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)
	setupSchema(t, s)

	queryRes, err := query(s, data.NewList(`SELECT id FROM users`))
	assertErrNil(t, "query", err)

	list := queryRes.(data.List)
	rowsStruct := list.Get(0).(*data.Struct)
	rowsST := makeRowsST(rowsStruct)

	// First close succeeds.
	closeResult, err := rowsClose(rowsST, data.NewList())
	assertErrNil(t, "first rowsClose go-error", err)

	if lErr := listErr(closeResult); lErr != nil {
		t.Fatalf("expected nil error on first rowsClose, got %v", lErr)
	}

	// rowsNext on a closed cursor must return false, not panic or error.
	hasNext, err := rowsNext(rowsST, data.NewList())
	assertErrNil(t, "rowsNext after close go-error", err)

	if hasNext != false {
		t.Fatal("expected rowsNext to return false after close")
	}

	// rowsHeadings returns ErrDatabaseClientClosed as a Go error.
	_, err = rowsHeadings(rowsST, data.NewList())
	if err == nil {
		t.Fatal("expected ErrDatabaseClientClosed from rowsHeadings after close, got nil")
	}

	// Second rowsClose embeds ErrDatabaseClientClosed in the returned list.
	closeResult2, _ := rowsClose(rowsST, data.NewList())

	if listErr(closeResult2) == nil {
		t.Fatal("expected ErrDatabaseClientClosed from second rowsClose, got nil")
	}
}

// Previously, query() used args.Elements()[1:args.Len()] in the non-tx path
// but args.Elements()[1:] in the tx path. The non-tx path has been normalized
// to args.Elements()[1:] to match. This test verifies both paths work correctly.
func TestBug_QueryArgSliceInconsistency_BothPathsWork(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, _ := makeClientST(t, dbPath)
	setupSchema(t, s)

	// Non-tx path: query with a parameter.
	res, err := query(s, data.NewList(`SELECT name FROM users WHERE id=?`, 1))
	assertErrNil(t, "non-tx parameterized query", err)

	list := res.(data.List)
	rowsStruct := list.Get(0).(*data.Struct)
	rowsST := makeRowsST(rowsStruct)
	_, _ = rowsClose(rowsST, data.NewList())

	// Tx path: same query inside a transaction.
	_, err = begin(s, data.NewList())
	assertErrNil(t, "begin", err)

	res, err = query(s, data.NewList(`SELECT name FROM users WHERE id=?`, 2))
	assertErrNil(t, "tx parameterized query", err)

	list = res.(data.List)
	rowsStruct = list.Get(0).(*data.Struct)
	rowsST = makeRowsST(rowsStruct)
	_, _ = rowsClose(rowsST, data.NewList())

	_, _ = rollback(s, data.NewList())
}

// Previously, rowsScan returned data.NewMapFromMap (*data.Map) in struct mode
// while queryResult returned data.NewStructFromMap (*data.Struct). This caused
// a type mismatch visible to callers. rowsScan now uses NewStructFromMap so
// both paths return *data.Struct consistently.
func TestBug_StructModeMismatch_RowsScanVsQueryResult(t *testing.T) {
	dbPath, cleanup := makeTestDB(t)
	defer cleanup()

	s, clientStruct := makeClientST(t, dbPath)
	setupSchema(t, s)
	clientStruct.SetAlways(asStructFieldName, true)

	// queryResult returns *data.Struct per row.
	qrRes, err := queryResult(s, data.NewList(`SELECT id, name, age FROM users WHERE id=1`))
	assertErrNil(t, "queryResult struct mode", err)

	qrList := qrRes.(data.List)
	qrArr := qrList.Get(0).(*data.Array)
	qrFirst, _ := qrArr.Get(0)

	if _, ok := qrFirst.(*data.Struct); !ok {
		t.Fatalf("queryResult struct mode: expected *data.Struct, got %T", qrFirst)
	}

	// rowsScan must also return *data.Struct per row (after the bug fix).
	queryRes, err := query(s, data.NewList(`SELECT id, name, age FROM users WHERE id=1`))
	assertErrNil(t, "query for scan test", err)

	qList := queryRes.(data.List)
	rowsStruct := qList.Get(0).(*data.Struct)
	rowsStruct.SetAlways(dbFieldName, clientStruct)
	rowsST := makeRowsST(rowsStruct)

	_, _ = rowsNext(rowsST, data.NewList())
	scanRes, err := rowsScan(rowsST, data.NewList())
	assertErrNil(t, "rowsScan struct mode", err)

	scanList := scanRes.(data.List)
	scanFirst := scanList.Get(0)

	if _, ok := scanFirst.(*data.Struct); !ok {
		t.Fatalf("rowsScan struct mode: expected *data.Struct, got %T — bug may not be fixed", scanFirst)
	}

	_, _ = rowsClose(rowsST, data.NewList())
}
