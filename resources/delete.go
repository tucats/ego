package resources

import (
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

const (
	whereClause = " where "
	andClause   = " and "
)

// Delete removes one or more resources from the data. If you do not specify
// a filter then all resources of the given handle are deleted. The filters
// are cumulative (that is, the resources select must match ALL the filters).
//
// The function returns the number of items deleted, and any error condition.
func (r *ResHandle) Delete(filters ...*Filter) (int64, error) {
	var (
		err   error
		count int64
	)

	if r.Err != nil {
		return 0, r.Err
	}

	if r.Database == nil {
		return 0, ErrDatabaseNotOpen
	}

	sql := r.deleteRowSQL()

	for index, filter := range filters {
		if index == 0 {
			sql = sql + whereClause
		} else {
			sql = sql + andClause
		}

		sql = sql + filter.Generate()
	}

	ui.Log(ui.ResourceLogger, "[0] Delete: %s", sql)

	result, err := r.Database.Exec(sql)
	if err == nil {
		count, err = result.RowsAffected()
	}

	return count, err
}

// DeleteOne deletes a single resource using it's primary key value.
func (r *ResHandle) DeleteOne(key interface{}) error {
	keyField := r.PrimaryKey()
	if keyField == "" {
		return errors.ErrNotFound
	}

	count, err := r.Delete(r.Equals(keyField, key))
	if err != nil {
		return err
	}

	if count == 0 {
		return errors.ErrNotFound.Context(key)
	}

	return nil
}
