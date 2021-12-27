package dbtables

const (
	tablesQueryString       = `SELECT table_name FROM information_schema.tables WHERE table_schema = '{{schema}}' ORDER BY table_name;`
	tableMetadataQuerySting = `SELECT * FROM {{schema}}.{{table}} WHERE 1=0;`
	tableDeleteString       = `DROP TABLE {{schema}}.{{table}};`
	createPermissionString  = `CREATE TABLE IF NOT EXISTS admin.privileges(username CHAR VARYING, tablename CHAR VARYING, permissions CHAR VARYING)`
	permissionsSelectString = `SELECT permissions FROM admin.privileges WHERE username = $1 and tablename = $2`
	permissionsDeleteString = `DELETE FROM admin.privileges WHERE tablename = $1`
	permissionsInsertString = `INSERT INTO admin.privileges (username, tablename, permissions) VALUES($1, $2, $3)`
	createSchemaString      = `CREATE SCHEMA IF NOT EXISTS {{schema}}`

	selectVerb = "SELECT"
	deleteVerb = "DELETE"
	updateVerb = "UPDATE"
	insertVerb = "INSERT"

	rowIDName              = "_row_id_"
	sqlPseudoTable         = "@sql"
	permissionsPseudoTable = "@permissions"
)
