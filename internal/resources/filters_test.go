package resources

import (
	"os"
	"testing"

	"github.com/google/uuid"
)

// Tests for Filter/Generate parameter binding (see filters.go, read.go,
// update.go, delete.go). Before this fix, a Filter's Value was quoted and
// concatenated directly into the SQL text, and any string/UUID value
// containing a "'" was rejected outright with ErrSQLInjection rather than
// safely bound. These tests use a real (temporary, file-backed) SQLite
// database to prove two things end to end:
//
//  1. A value containing a "'" (e.g. a name like "O'Brien") is no longer
//     rejected -- it round-trips correctly through Read/Update/Delete.
//  2. A value that LOOKS like a SQL injection payload is bound as an inert
//     string, not executed as SQL: it matches nothing (rather than, say,
//     matching every row the way a real `' OR '1'='1` injection would).
// ID is deliberately the first field: the resources package always treats
// column 0 as the table's primary (unique) key (see modifiers.go), and this
// test suite needs to insert multiple rows sharing the same Name so that
// TestFilter_MultipleFilters_PlaceholderNumberingIsCorrect can prove the
// Name+Age filter combination narrows to exactly one row.
type filterTestObject struct {
	ID     uuid.UUID
	Name   string
	Age    int
	Active bool
}

// openFilterTestResource opens a fresh temporary SQLite-backed resource
// table for filterTestObject and registers cleanup of both the table and
// the backing file when the test completes.
func openFilterTestResource(t *testing.T) *ResHandle {
	t.Helper()

	dbname := "testing-filters-" + uuid.New().String() + ".db"
	connection := "sqlite://" + dbname

	r, err := Open(filterTestObject{}, "filter_testing", connection)
	if err != nil {
		t.Fatalf("error opening connection: %v", err)
	}

	if err := r.CreateIf(); err != nil {
		t.Fatalf("error creating table: %v", err)
	}

	t.Cleanup(func() {
		if err := r.DropAllResources(); err != nil {
			t.Errorf("error dropping table: %v", err)
		}

		_ = os.Remove(dbname)
	})

	return r
}

// TestFilter_ValueWithEmbeddedQuote_Read is the direct regression test: a
// name containing a "'" must be usable as a filter value for Read, and must
// match exactly the row with that literal name -- proving the value is
// bound as a parameter rather than rejected or mishandled as SQL text.
func TestFilter_ValueWithEmbeddedQuote_Read(t *testing.T) {
	r := openFilterTestResource(t)

	if err := r.Insert(filterTestObject{Name: "O'Brien", Age: 40, ID: uuid.New()}); err != nil {
		t.Fatalf("error inserting row: %v", err)
	}

	if err := r.Insert(filterTestObject{Name: "Smith", Age: 41, ID: uuid.New()}); err != nil {
		t.Fatalf("error inserting row: %v", err)
	}

	if r.Err != nil {
		t.Fatalf("unexpected error constructing filter: %v", r.Err)
	}

	items, err := r.Read(r.Equals("Name", "O'Brien"))
	if err != nil {
		t.Fatalf("error reading table: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected exactly 1 matching row, got %d", len(items))
	}

	object, ok := items[0].(*filterTestObject)
	if !ok {
		t.Fatalf("value returned was of wrong type: %#v", items[0])
	}

	if object.Name != "O'Brien" {
		t.Errorf("got name %q, want %q", object.Name, "O'Brien")
	}
}

// TestFilter_InjectionLookingValue_MatchesNothing uses a classic SQL
// injection payload as a filter value. Since the value is bound as a query
// parameter rather than concatenated into the SQL text, it is compared as
// an ordinary (and non-matching) string -- it must NOT behave like the
// injected `OR '1'='1` clause it resembles, which would otherwise make the
// query match every row in the table.
func TestFilter_InjectionLookingValue_MatchesNothing(t *testing.T) {
	r := openFilterTestResource(t)

	if err := r.Insert(filterTestObject{Name: "Tom", Age: 50, ID: uuid.New()}); err != nil {
		t.Fatalf("error inserting row: %v", err)
	}

	if err := r.Insert(filterTestObject{Name: "Mark", Age: 51, ID: uuid.New()}); err != nil {
		t.Fatalf("error inserting row: %v", err)
	}

	payload := "x' OR '1'='1"

	items, err := r.Read(r.Equals("Name", payload))
	if err != nil {
		t.Fatalf("error reading table: %v", err)
	}

	if len(items) != 0 {
		t.Fatalf("injection-looking value matched %d rows, want 0 (value should be bound as an inert string)", len(items))
	}
}

// TestFilter_ValueWithEmbeddedQuote_Update verifies that Update's WHERE
// clause correctly binds a filter value containing a "'", AND that the
// SET-clause placeholders (from updateSQL, $1..$len(columns)) and the
// WHERE-clause placeholder added for the filter do not collide -- this is
// the trickiest part of the parameter-numbering fix, since both share the
// same statement's placeholder sequence.
func TestFilter_ValueWithEmbeddedQuote_Update(t *testing.T) {
	r := openFilterTestResource(t)

	if err := r.Insert(filterTestObject{Name: "O'Brien", Age: 40, ID: uuid.New()}); err != nil {
		t.Fatalf("error inserting row: %v", err)
	}

	err := r.Update(filterTestObject{Name: "O'Brien", Age: 99, ID: uuid.New()}, r.Equals("Name", "O'Brien"))
	if err != nil {
		t.Fatalf("error updating row: %v", err)
	}

	items, err := r.Read(r.Equals("Name", "O'Brien"))
	if err != nil {
		t.Fatalf("error reading table: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected exactly 1 matching row after update, got %d", len(items))
	}

	object, ok := items[0].(*filterTestObject)
	if !ok {
		t.Fatalf("value returned was of wrong type: %#v", items[0])
	}

	if object.Age != 99 {
		t.Errorf("got age %d, want 99 (update did not take effect correctly)", object.Age)
	}
}

// TestFilter_ValueWithEmbeddedQuote_Delete verifies Delete's WHERE clause
// correctly binds and matches a filter value containing a "'", deleting
// exactly the intended row and leaving others untouched.
func TestFilter_ValueWithEmbeddedQuote_Delete(t *testing.T) {
	r := openFilterTestResource(t)

	if err := r.Insert(filterTestObject{Name: "O'Brien", Age: 40, ID: uuid.New()}); err != nil {
		t.Fatalf("error inserting row: %v", err)
	}

	if err := r.Insert(filterTestObject{Name: "Smith", Age: 41, ID: uuid.New()}); err != nil {
		t.Fatalf("error inserting row: %v", err)
	}

	count, err := r.Delete(r.Equals("Name", "O'Brien"))
	if err != nil {
		t.Fatalf("error deleting row: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected exactly 1 row deleted, got %d", count)
	}

	remaining, err := r.Read()
	if err != nil {
		t.Fatalf("error reading table: %v", err)
	}

	if len(remaining) != 1 {
		t.Fatalf("expected exactly 1 row remaining, got %d", len(remaining))
	}

	object, ok := remaining[0].(*filterTestObject)
	if !ok {
		t.Fatalf("value returned was of wrong type: %#v", remaining[0])
	}

	if object.Name != "Smith" {
		t.Errorf("got name %q, want %q (wrong row was deleted)", object.Name, "Smith")
	}
}

// TestFilter_MultipleFilters_PlaceholderNumberingIsCorrect verifies that
// combining two filters on a Read (joined with AND) numbers their $N
// placeholders sequentially ($1, $2, ...) rather than colliding, and that
// one of the two values contains a "'" to keep the embedded-quote coverage
// in the multi-filter case too.
func TestFilter_MultipleFilters_PlaceholderNumberingIsCorrect(t *testing.T) {
	r := openFilterTestResource(t)

	if err := r.Insert(filterTestObject{Name: "O'Brien", Age: 40, ID: uuid.New()}); err != nil {
		t.Fatalf("error inserting row: %v", err)
	}

	if err := r.Insert(filterTestObject{Name: "O'Brien", Age: 41, ID: uuid.New()}); err != nil {
		t.Fatalf("error inserting row: %v", err)
	}

	items, err := r.Read(r.Equals("Name", "O'Brien"), r.Equals("Age", 41))
	if err != nil {
		t.Fatalf("error reading table: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected exactly 1 matching row, got %d", len(items))
	}

	object, ok := items[0].(*filterTestObject)
	if !ok {
		t.Fatalf("value returned was of wrong type: %#v", items[0])
	}

	if object.Age != 41 {
		t.Errorf("got age %d, want 41", object.Age)
	}
}
