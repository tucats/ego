package resources

import "strings"

// PrimaryKey marks a column as the primary key of the database table.
func (r *ResHandle) PrimaryKey(name string) *ResHandle {
	if r.Columns != nil {
		for i := 0; i < len(r.Columns); i++ {
			column := r.Columns[i]

			if strings.EqualFold(column.Name, name) {
				r.Columns[i].Primary = true
			}
		}
	}

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
