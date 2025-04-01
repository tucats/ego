package resources

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

// Update updates one or more resources in the database table for
// the given resource. If there are no filters, then all values in
// the table are updated to match the value object passed as the
// first parameter.
//
// The operation replaces each object in the table that matches the
// criteria with the given object. Note that it would be *very*
// unusual not to specify filters such that the operation is done
// on a single object.
func (r *ResHandle) Update(v interface{}, filters ...*Filter) error {
	var err error

	if r.Err != nil {
		return r.Err
	}

	sql := r.updateSQL()

	for index, filter := range filters {
		if index == 0 {
			sql = sql + whereClause
		} else {
			sql = sql + andClause
		}

		sql = sql + filter.Generate()
	}

	items := r.explode(v)

	ui.Log(ui.ResourceLogger, "resource.update", ui.A{
		"sql": sql})
	ui.Log(ui.ResourceLogger, "resource.parms", ui.A{
		"list": items})

	_, err = r.Database.Exec(sql, items...)

	return err
}

// UpdateOne updates the single object that matches the provided
// object's primary key value. If the primary key is not set, or
// the object is not found, then an error is reported.
func (r *ResHandle) UpdateOne(v interface{}) error {
	keyIndex := r.PrimaryKeyIndex()
	if keyIndex < 0 {
		return errors.ErrNotFound
	}

	items := r.explode(v)

	return r.Update(v, r.Equals(r.Columns[keyIndex].SQLName, items[keyIndex]))
}
