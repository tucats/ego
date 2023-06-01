package resources

import "github.com/tucats/ego/app-cli/ui"

func (r *ResHandle) Insert(v interface{}) error {
	var err error

	sql := r.insertSQL()
	items := r.explode(v)

	ui.Log(ui.DBLogger, "[0] Resource insert: %s", sql)

	_, err = r.Database.Exec(sql, items...)

	return err
}
