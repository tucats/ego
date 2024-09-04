package resources

import "strings"

// SetPrimaryKey marks a column as the primary key of the database table.
// If there was previously a primary key, it is removed. Note that this
// operation must be done before the Create() method is called, which
// will set the key field in the database.
func (r *ResHandle) SetPrimaryKey(name string) *ResHandle {
	if r.Columns != nil {
		for i := 0; i < len(r.Columns); i++ {
			r.Columns[i].Primary = false

			if strings.EqualFold(r.Columns[i].Name, name) {
				r.Columns[i].Primary = true
			}
		}
	}

	return r
}

// Determine which field in the columns list is used as the primary key.
// If there is a column named "id" or "name" then that is the default
// key field. Otherwise, the first column is used as the key field.
func (r *ResHandle) SetDefaultPrimaryKey() *ResHandle {
	// Is there a field already marked as the primary key?
	for _, column := range r.Columns {
		if column.Primary {
			return r
		}
	}

	// Is there a field named "id" that we will assume is the key?
	for index, column := range r.Columns {
		if column.Name == "id" {
			r.Columns[index].Primary = true
			
			return r
		}
	}

	// Is there a field named "name" that we can assume is the key?
	for index, column := range r.Columns {
		if column.Name == "name" {
			r.Columns[index].Primary = true

			return r
		}
	}

	// No primary was found, so assume the first column is the key.
	r.Columns[0].Primary = true

	return r
}

// Nullable marks a column as being nullable in the database table.
func (r *ResHandle) Nullable(name string) *ResHandle {
	if r.Columns != nil {
		for i := 0; i < len(r.Columns); i++ {
			column := r.Columns[i]

			if strings.EqualFold(column.Name, name) {
				r.Columns[i].Nullable = true
			}
		}
	}

	return r
}

// SetSQLType marks a column as being nullable in the database table.
func (r *ResHandle) SetSQLType(name, typeString string) *ResHandle {
	if r.Columns != nil {
		for i := 0; i < len(r.Columns); i++ {
			column := r.Columns[i]

			if strings.EqualFold(column.Name, name) {
				r.Columns[i].SQLType = typeString
			}
		}
	}

	return r
}

// SetSQLType marks a column as being nullable in the database table.
func (r *ResHandle) SetSQLName(name, sqlName string) *ResHandle {
	if r.Columns != nil {
		for i := 0; i < len(r.Columns); i++ {
			column := r.Columns[i]

			if strings.EqualFold(column.Name, name) {
				r.Columns[i].SQLName = sqlName
			}
		}
	}

	return r
}
