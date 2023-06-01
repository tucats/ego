package resources

import "errors"

func (r *ResHandle) Create() error {
	var err error

	if r.Database == nil {
		return errors.New("database not open")
	}

	sql := r.createTableSQL()
	_, err = r.Database.Exec(sql)

	return err
}

func (r *ResHandle) CreateIf() error {
	var err error

	if r.Database == nil {
		return errors.New("database not open")
	}

	sql := r.doesTableExistSQL()
	rows, err := r.Database.Query(sql)

	if rows != nil {
		defer rows.Close()
	}

	if err != nil {
		return r.Create()
	}

	return err
}
