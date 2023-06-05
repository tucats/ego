package resources

import "github.com/tucats/ego/app-cli/ui"

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
