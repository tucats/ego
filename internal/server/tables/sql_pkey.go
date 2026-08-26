package tables

import (
	"fmt"
	"strings"

	"github.com/tucats/ego/internal/caches"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/tables/database"
	"github.com/tucats/ego/internal/sqlparse"
	"github.com/tucats/ego/internal/sqlparse/ast"
	egostrings "github.com/tucats/ego/internal/util/strings"
)

// This file adds the "pkey"/"ptable" analysis to the @sql endpoint's RowSet
// response (SQLTransaction/readRowDataTx, in sql.go): when the query that
// produced the RowSet was a plain, single-table SELECT, and one of its
// unaliased result columns is a single-column unique key of that table, a
// client can safely use that column's value to target exactly one row of
// that table for a later single-row UPDATE or DELETE. Either or both of
// PKey/PTable are left blank whenever that cannot be determined with
// confidence -- a JOIN, a UNION/INTERSECT/EXCEPT, a WITH clause (whose CTE
// names sqlparse cannot distinguish from real tables -- see
// sqlparse.Sqlparse.Tables' own doc comment on the same limitation), a
// subquery source, or no qualifying column in the SELECT list.
//
// A column that belongs to a multi-column (composite) unique index or
// primary key is deliberately never reported as pkey: knowing just that one
// column's value is not enough, on its own, to identify a single row.

// analyzeSingleTableSelect inspects q -- the exact SQL text that produced
// columnNames via readRowDataTx -- and reports the PKey/PTable pair for the
// RowSet response. Any failure along the way (the statement doesn't parse,
// it isn't a single-table SELECT, or the unique-key catalog lookup errors)
// simply yields blank results rather than failing the request: this
// analysis is a best-effort addition to a query that has already succeeded.
func analyzeSingleTableSelect(session *router.Session, db *database.Database, q string, columnNames []string) (pkey, ptable string) {
	p, err := sqlparse.New(q, sqlDialect(db.Provider))
	if err != nil {
		return "", ""
	}

	ref, resultColumns, ok := singleSourceSelect(p.Statement())
	if !ok {
		return "", ""
	}

	ptable = tableRefFullName(ref)

	pk, unique, err := singleColumnUniqueKeys(session, db, ref)
	if err != nil {
		return "", ptable
	}

	candidates := resultColumnCandidates(resultColumns, columnNames)
	pkey = choosePrimaryKey(candidates, columnNames, pk, unique)

	return pkey, ptable
}

// singleSourceSelect reports the single table stmt reads from, and its
// result-column list, when stmt is a plain, single-table SELECT: no WITH
// clause, no UNION/INTERSECT/EXCEPT (*ast.CompoundSelect), and a FROM
// clause that is exactly one bare table reference -- no JOIN, no subquery
// source. ok is false, and table/columns are nil, for anything else.
func singleSourceSelect(stmt ast.Statement) (table *ast.TableRef, columns []*ast.ResultColumn, ok bool) {
	sel, isSelect := stmt.(*ast.SelectStmt)
	if !isSelect || sel.With != nil {
		return nil, nil, false
	}

	core, isCore := sel.Select.(*ast.SelectCore)
	if !isCore || len(core.From) != 1 {
		return nil, nil, false
	}

	ref, isTable := core.From[0].(*ast.TableRef)
	if !isTable {
		return nil, nil, false
	}

	return ref, core.Columns, true
}

// tableRefFullName renders ref the same way sqlparse.Sqlparse.Tables()
// does: schema-qualified when the query wrote it that way, the bare name
// otherwise.
func tableRefFullName(ref *ast.TableRef) string {
	if ref.Schema != "" {
		return ref.Schema + "." + ref.Name
	}

	return ref.Name
}

// resultColumnCandidates returns, in SELECT-list order, the column names
// from columns that are plain and unaliased -- and so name a real column of
// the source table rather than a computed or renamed value. A bare "*" (or
// "table.*") as the sole result column contributes every one of
// driverColumns, the query's actual output columns, which -- absent any
// alias elsewhere in the list, guaranteed here because a lone "*" is the
// only entry -- are exactly the table's own column names in the table's own
// order. A "*" combined with other result columns is not expanded (there is
// no reliable way to line its expansion up positionally against
// driverColumns without the table's full column list), so only the other,
// explicit column references in such a list still contribute.
func resultColumnCandidates(columns []*ast.ResultColumn, driverColumns []string) []string {
	if len(columns) == 1 {
		if _, isStar := columns[0].Expr.(*ast.StarExpr); isStar {
			return append([]string(nil), driverColumns...)
		}
	}

	var candidates []string

	for _, col := range columns {
		if col.Alias != "" {
			continue
		}

		if ref, isColumn := col.Expr.(*ast.ColumnRef); isColumn {
			candidates = append(candidates, ref.Column)
		}
	}

	return candidates
}

// choosePrimaryKey picks the column to report as PKey from candidates (in
// SELECT-list order, as parsed from the SQL text): the table's own
// single-column primary key (pk) if it is among candidates, otherwise the
// first candidate that is at least a single-column UNIQUE key (unique). pk
// and the keys of unique are lower-cased (see singleColumnUniqueKeys); the
// match against candidates and driverColumns is therefore case-insensitive,
// but the value returned is always driverColumns' own literal spelling, so
// it is guaranteed to be usable as a key into the RowSet's Rows maps.
func choosePrimaryKey(candidates []string, driverColumns []string, pk string, unique map[string]bool) string {
	driverNameFor := func(name string) (string, bool) {
		lower := strings.ToLower(name)

		for _, c := range driverColumns {
			if strings.ToLower(c) == lower {
				return c, true
			}
		}

		return "", false
	}

	if pk != "" {
		for _, c := range candidates {
			if strings.ToLower(c) == pk {
				if driverName, found := driverNameFor(c); found {
					return driverName
				}
			}
		}
	}

	for _, c := range candidates {
		if unique[strings.ToLower(c)] {
			if driverName, found := driverNameFor(c); found {
				return driverName
			}
		}
	}

	return ""
}

// uniqueKeyInfo is the cached shape of singleColumnUniqueKeys' result --
// see that function's own doc comment for what PK/Unique mean. Both fields
// are read-only once cached (see choosePrimaryKey, the sole reader of
// Unique): every candidate lookup only ever indexes into the map, never
// mutates it, so the same cached map instance is safe to hand back on every
// hit rather than copying it per call.
type uniqueKeyInfo struct {
	PK     string
	Unique map[string]bool
}

// uniqueKeysCacheKey builds singleColumnUniqueKeys' cache key for ref's
// table against db, following the same "identity/dsn/table" shape
// getColumnInfo (tables.go) already uses for its own caches.SchemaCache
// entries -- db.User leads the key (the DSN's resolved schema for
// PostgreSQL, since database.Open -- not the caller's Ego identity; see
// that function's doc comment), so entries naturally separate by schema
// even when ref itself did not write one. The "pkey:" prefix keeps this
// disjoint from getColumnInfo's own keys sharing the same cache class.
func uniqueKeysCacheKey(db *database.Database, ref *ast.TableRef) string {
	dsn := db.DSN
	if dsn == "" {
		dsn = "-"
	}

	return "pkey:" + db.User + "/" + dsn + "/" + tableRefFullName(ref)
}

// singleColumnUniqueKeys reports ref's table's single-column primary key
// column name (pk, lower-cased; "" if the table has no primary key or its
// primary key spans more than one column) and the set of columns (also
// lower-cased) individually covered by a single-column UNIQUE index or
// constraint -- which always includes pk, when pk is set.
//
// The result is cached in caches.SchemaCache, keyed by uniqueKeysCacheKey,
// since it costs a real database round-trip per call otherwise (one
// system-catalog query for PostgreSQL, or a PRAGMA sequence for SQLite) --
// and analyzeSingleTableSelect runs this for every single-table @sql SELECT.
// A statement that can invalidate the answer (CREATE/ALTER/DROP TABLE,
// CREATE/DROP INDEX) purges the whole cache class on commit; see
// isSchemaAlteringKind in sql_permissions.go and its callers.
func singleColumnUniqueKeys(session *router.Session, db *database.Database, ref *ast.TableRef) (pk string, unique map[string]bool, err error) {
	cacheKey := uniqueKeysCacheKey(db, ref)

	if cached, ok := caches.Find(caches.SchemaCache, cacheKey); ok {
		if info, ok := cached.(uniqueKeyInfo); ok {
			return info.PK, info.Unique, nil
		}
	}

	switch db.Provider {
	case defs.PostgresProvider:
		pk, unique, err = postgresSingleColumnUniqueKeys(session, db, ref)
	case defs.SqliteProvider:
		pk, unique, err = sqliteSingleColumnUniqueKeys(db, ref)
	default:
		return "", map[string]bool{}, nil
	}

	if err == nil {
		caches.Add(caches.SchemaCache, cacheKey, uniqueKeyInfo{PK: pk, Unique: unique})
	}

	return pk, unique, err
}

// postgresSingleColumnUniqueKeys implements singleColumnUniqueKeys for a
// PostgreSQL-backed DSN. When ref did not write a schema, db.User (the DSN's
// configured schema, not the Ego identity) is used as the default schema,
// matching getPostgresColumnMetadata's own convention in describe.go.
func postgresSingleColumnUniqueKeys(session *router.Session, db *database.Database, ref *ast.TableRef) (string, map[string]bool, error) {
	schema := ref.Schema
	if schema == "" {
		schema = db.User
	}

	_ = session.ID

	rows, err := db.Query(singleColumnUniqueKeysQuery, schema, ref.Name)
	if err != nil {
		return "", nil, err
	}

	defer rows.Close()

	pk := ""
	unique := map[string]bool{}

	for rows.Next() {
		var (
			name string
			isPK bool
		)

		if err := rows.Scan(&name, &isPK); err != nil {
			return "", nil, err
		}

		name = strings.ToLower(name)
		unique[name] = true

		if isPK {
			pk = name
		}
	}

	return pk, unique, rows.Err()
}

// sqliteSingleColumnUniqueKeys implements singleColumnUniqueKeys for a
// SQLite-backed DSN.
//
// A single-column PRIMARY KEY declared as INTEGER PRIMARY KEY is an alias
// for sqlite's own rowid, and -- unlike an ordinary UNIQUE constraint --
// gets no separate entry in PRAGMA index_list; PRAGMA table_info's own "pk"
// column (1-based ordinal within the primary key, 0 when the column is not
// part of it) is the only way to find it, and is also how any other
// single-column PRIMARY KEY is recognized here. A composite primary key
// reports more than one column with pk > 0, and is deliberately excluded
// (see this file's package comment).
func sqliteSingleColumnUniqueKeys(db *database.Database, ref *ast.TableRef) (string, map[string]bool, error) {
	tableName := egostrings.SQLIdentifier(ref.Name)
	unique := map[string]bool{}
	pk := ""

	infoRows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return "", nil, err
	}

	pkColumns := []string{}

	for infoRows.Next() {
		var (
			cid          int
			name         string
			datatype     string
			notNull      bool
			defaultValue any
			pkOrdinal    int
		)

		if err := infoRows.Scan(&cid, &name, &datatype, &notNull, &defaultValue, &pkOrdinal); err != nil {
			infoRows.Close()

			return "", nil, err
		}

		if pkOrdinal > 0 {
			pkColumns = append(pkColumns, strings.ToLower(name))
		}
	}

	infoRows.Close()

	if len(pkColumns) == 1 {
		pk = pkColumns[0]
		unique[pk] = true
	}

	listRows, err := db.Query(fmt.Sprintf("PRAGMA index_list(%s)", tableName))
	if err != nil {
		return pk, unique, err
	}

	indexes := []string{}

	for listRows.Next() {
		var (
			seq      int
			name     string
			isUnique bool
			origin   string
			partial  bool
		)

		if err := listRows.Scan(&seq, &name, &isUnique, &origin, &partial); err != nil {
			listRows.Close()

			return pk, unique, err
		}

		if isUnique {
			indexes = append(indexes, name)
		}
	}

	listRows.Close()

	for _, index := range indexes {
		indexInfoRows, err := db.Query(fmt.Sprintf("PRAGMA index_info(%s)", egostrings.SQLIdentifier(index)))
		if err != nil {
			return pk, unique, err
		}

		cols := []string{}

		for indexInfoRows.Next() {
			var (
				seqno int
				cid   int
				name  string
			)

			if err := indexInfoRows.Scan(&seqno, &cid, &name); err != nil {
				indexInfoRows.Close()

				return pk, unique, err
			}

			cols = append(cols, name)
		}

		indexInfoRows.Close()

		if len(cols) == 1 {
			unique[strings.ToLower(cols[0])] = true
		}
	}

	return pk, unique, nil
}
