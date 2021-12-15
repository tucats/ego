package dbtables

const (
	tablesQueryString = `SELECT table_name FROM information_schema.tables WHERE table_schema = '{{schema}}' ORDER BY table_name;`
)
