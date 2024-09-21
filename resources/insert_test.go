package resources

import (
	"os"
	"testing"

	"github.com/google/uuid"
)

// Insert values into a resource, and read them back. When done,
// drop the resource table.
func Test_insert(t *testing.T) {
	type objectType struct {
		Name   string
		Age    int
		Active bool
		ID     uuid.UUID
	}

	dbname := "testing-" + uuid.New().String() + ".db"
	connection := "sqlite3://" + dbname

	value := objectType{}

	r, err := Open(value, "testing", connection)
	if err != nil {
		t.Errorf("error opening connection, %v", err)
	}

	err = r.CreateIf()
	if err != nil {
		t.Errorf("error creating table, %v", err)
	}

	// Write some rows
	err = r.Insert(objectType{
		Name:   "Tom",
		Age:    63,
		Active: true,
		ID:     uuid.New(),
	})

	if err != nil {
		t.Errorf("error reading table, %v", err)
	}

	err = r.Insert(objectType{
		Name:   "Mark",
		Age:    62,
		Active: true,
		ID:     uuid.New(),
	})

	if err != nil {
		t.Errorf("error reading table, %v", err)
	}

	err = r.Insert(objectType{
		Name:   "Buddy",
		Age:    65,
		Active: false,
		ID:     uuid.New(),
	})

	if err != nil {
		t.Errorf("error reading table, %v", err)
	}

	// Read back one row
	items, err := r.Read(r.Equals("Name", "Tom"))
	if err != nil {
		t.Errorf("error reading table, %v", err)
	}

	if items == nil {
		t.Errorf("no value returned from read")
	}

	if len(items) == 0 {
		t.Errorf("no value found in filter from read")
	}

	if object, ok := items[0].(*objectType); ok {
		t.Logf("Found item, name = %v", object.Name)
	} else {
		t.Errorf("value returned was of wrong type: %#v", items[0])
	}

	if err := r.DropAllResources(); err != nil {
		t.Errorf("error dropping table, %v", err)
	} else {
		_ = os.Remove(dbname)
	}
}
