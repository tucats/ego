package dbtables

const (
	tablesListQuery             = `SELECT table_name FROM information_schema.tables WHERE table_schema = '{{schema}}' ORDER BY table_name`
	tableMetadataQuery          = `SELECT * FROM {{schema}}.{{table}} WHERE 1=0`
	tableDeleteQuery            = `DROP TABLE {{schema}}.{{table}};`
	createSchemaQuery           = `CREATE SCHEMA IF NOT EXISTS {{schema}}`
	permissionsCreateTableQuery = `CREATE TABLE IF NOT EXISTS admin.privileges(username CHAR VARYING, tablename CHAR VARYING, permissions CHAR VARYING)`
	permissionsSelectQuery      = `SELECT permissions FROM admin.privileges WHERE username = $1 and tablename = $2`
	permissionsDeleteQuery      = `DELETE FROM admin.privileges WHERE username=$1 AND tablename = $2`
	permissionsDeleteAllQuery   = `DELETE FROM admin.privileges WHERE tablename = $1`
	permissionsInsertQuery      = `INSERT INTO admin.privileges (username, tablename, permissions) VALUES($1, $2, $3)`
	permissionsUpdateQuery      = `UPDATE admin.privileges SET permissions=$3 WHERE username=$1 AND tablename=$2`
	rowCountQuery               = `SELECT COUNT(*) FROM "{{schema}}"."{{table}}"`

	selectVerb = "SELECT"
	deleteVerb = "DELETE"
	updateVerb = "UPDATE"
	insertVerb = "INSERT"

	sqlPseudoTable         = "@sql"
	permissionsPseudoTable = "@permissions"
)
