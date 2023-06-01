package resources

func (r *ResHandle) Insert(v interface{}) error {
	var err error

	sql := r.insertSQL()
	items := r.explode(v)

	_, err = r.Database.Exec(sql, items...)

	return err
}
