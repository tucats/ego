package resources

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

func (r *ResHandle) Insert(v any) error {
	var err error

	if r == nil {
		return errors.ErrNoResourceHandle
	}

	if r.Err != nil {
		return r.Err
	}

	sql := r.insertSQL()
	items := r.explode(v)

	ui.Log(ui.ResourceLogger, "resource.insert", ui.A{
		"sql": sql})
	ui.Log(ui.ResourceLogger, "resource.parms", ui.A{
		"list": items})

	_, err = r.Database.Exec(sql, items...)

	return err
}
