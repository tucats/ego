package dbtables

const (
	tablesQueryString       = `SELECT table_name FROM information_schema.tables WHERE table_schema = '{{schema}}' ORDER BY table_name;`
	tableMetadataQuerySting = `SELECT * FROM {{schema}}.{{table}} WHERE 1=0;`
	tableDeleteString       = `DROP TABLE {{schema}}.{{table}};`

	selectVerb = "SELECT"
	deleteVerb = "DELETE"
	updateVerb = "UPDATE"
)

type DBColumn struct {
	Name     string
	Type     string
	Size     int
	Nullable bool
}
