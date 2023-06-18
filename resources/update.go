package resources

import "github.com/tucats/ego/app-cli/ui"

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

	sql := r.updateSQL()

	for index, filter := range filters {
		if index == 0 {
			sql = sql + " where "
		} else {
			sql = sql + " and "
		}

		sql = sql + filter.Generate()
	}

	items := r.explode(v)

	ui.Log(ui.DBLogger, "[0] Resource update: %s", sql)

	_, err = r.Database.Exec(sql, items...)

	return err
}
