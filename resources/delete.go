package resources

import "github.com/tucats/ego/app-cli/ui"

func (r *ResHandle) Delete(filters ...*Filter) (int64, error) {
	var (
		err   error
		count int64
	)

	if r.Database == nil {
		return 0, ErrDatabaseNotOpen
	}

	sql := r.deleteRowSQL()

	for index, filter := range filters {
		if index == 0 {
			sql = sql + " where "
		} else {
			sql = sql + " and "
		}

		sql = sql + filter.Generate()
	}

	ui.Log(ui.DBLogger, "[0] Resource delete: %s", sql)

	result, err := r.Database.Exec(sql)
	if err == nil {
		count, err = result.RowsAffected()
	}

	return count, err
}
