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

	// Get a list of table columns that are nullable in the given schema.table.
	nullableColumnsQuery = `SELECT  c.table_schema, 
									c.table_name,
									c.column_name,
									case c.is_nullable
										when 'NO' then false
										when 'YES' then true
									end as nullable
									FROM information_schema.columns c
									JOIN information_schema.tables t
									ON c.table_schema = t.table_schema 
										AND c.table_name = t.table_name
									WHERE c.table_schema = '{{schema}}'
										AND c.table_name = '{{table}}'
										AND t.table_type = 'BASE TABLE' 
									ORDER BY table_schema,
										table_name,
										column_name; `

	// Get a list of the table columns that have UNIQUEness constraints.
	uniqueColumnsQuery = `SELECT a.attname 
							FROM   pg_index i  
							JOIN   pg_attribute a 
								ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey) 
							WHERE  i.indrelid = '{{schema}}.{{table}}'::regclass;   `

	selectVerb = "SELECT"
	deleteVerb = "DELETE"
	updateVerb = "UPDATE"
	insertVerb = "INSERT"

	SQLPseudoTable         = "@sql"
	permissionsPseudoTable = "@permissions"

	syntaxErrorPrefix = "SYNTAX-ERROR:"
)
