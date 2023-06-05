package resources

import (
	"testing"
)

func Test_read(t *testing.T) {
	type objectType struct {
		Name   string
		Age    int
		Active bool
	}

	connection := "sqlite3://users.db"

	value := objectType{}

	r, err := Open(value, "testing", connection)
	if err != nil {
		t.Errorf("error opening connection, %v", err)
	}

	err = r.CreateIf()
	if err != nil {
		t.Errorf("error creating table, %v", err)
	}

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
}
