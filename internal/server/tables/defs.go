package tables

import "github.com/tucats/ego/internal/defs"

const (
	tablesListQuery          = `SELECT table_name FROM information_schema.tables WHERE table_schema = $1 ORDER BY table_name`
	tableMetadataQuery       = `SELECT * FROM {{schema}}.{{table}} WHERE 1=0`
	tableSQLiteMetadataQuery = `SELECT * FROM {{table}} WHERE 1=0`
	tableDeleteQuery         = `DROP TABLE {{schema}}.{{table}};`
	createSchemaQuery        = `CREATE SCHEMA IF NOT EXISTS {{schema}}`
	rowCountQuery            = `SELECT COUNT(*) FROM "{{schema}}"."{{table}}"`
	rowCountSQLiteQuery      = `SELECT COUNT(*) FROM "{{table}}"`

	// Get a list of table columns that are nullable in the given schema.table.
	// $1 = schema name, $2 = table name (both unquoted bare names).
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
									WHERE c.table_schema = $1
										AND c.table_name = $2
										AND t.table_type = 'BASE TABLE'
									ORDER BY table_schema,
										table_name,
										column_name; `

	// Get a list of the table columns that have UNIQUEness constraints.
	// $1 = schema name, $2 = table name (both unquoted bare names).
	// Uses pg_class/pg_namespace rather than ::regclass so mixed-case names
	// resolve correctly without casting issues.
	uniqueColumnsQuery = `SELECT a.attname
							FROM   pg_index i
							JOIN   pg_attribute a
								ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
							JOIN   pg_class c ON c.oid = i.indrelid
							JOIN   pg_namespace n ON n.oid = c.relnamespace
							WHERE  n.nspname = $1
							AND    c.relname = $2
							AND    i.indisunique = true;   `

	selectVerb = "SELECT"
	deleteVerb = "DELETE"
	updateVerb = "UPDATE"
	insertVerb = "INSERT"

	sqlPseudoTable = "@sql"

	syntaxErrorPrefix = "SYNTAX-ERROR:"
)

var providers = map[string]string{
	"sqlite3":    defs.SqliteProvider,
	"postgres":   defs.PostgresProvider,
	"sqlite":     defs.SqliteProvider,
	"postgresql": defs.PostgresProvider,
}
