package tables

import (
	"reflect"
	"testing"

	"github.com/tucats/ego/internal/sqlparse"
	"github.com/tucats/ego/internal/sqlparse/ast"
)

func mustParseSelect(t *testing.T, sql string) ast.Statement {
	t.Helper()

	p, err := sqlparse.New(sql, sqlparse.SQLite)
	if err != nil {
		t.Fatalf("failed to parse %q: %v", sql, err)
	}

	return p.Statement()
}

func Test_singleSourceSelect(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		wantOK     bool
		wantSchema string
		wantTable  string
		wantCols   int
	}{
		{
			name:      "simple single table",
			sql:       `SELECT id, name FROM t`,
			wantOK:    true,
			wantTable: "t",
			wantCols:  2,
		},
		{
			name:      "star single table",
			sql:       `SELECT * FROM t`,
			wantOK:    true,
			wantTable: "t",
			wantCols:  1,
		},
		{
			name:       "schema-qualified single table",
			sql:        `SELECT id FROM schema1.t`,
			wantOK:     true,
			wantSchema: "schema1",
			wantTable:  "t",
			wantCols:   1,
		},
		{
			name:   "join disqualifies",
			sql:    `SELECT t1.id FROM t1 JOIN t2 ON t1.id = t2.id`,
			wantOK: false,
		},
		{
			name:   "union disqualifies",
			sql:    `SELECT id FROM t1 UNION SELECT id FROM t2`,
			wantOK: false,
		},
		{
			name:   "with clause disqualifies",
			sql:    `WITH cte AS (SELECT id FROM t) SELECT id FROM cte`,
			wantOK: false,
		},
		{
			name:   "subquery source disqualifies",
			sql:    `SELECT id FROM (SELECT id FROM t) sub`,
			wantOK: false,
		},
		{
			name:   "non-select statement",
			sql:    `DELETE FROM t WHERE id = 1`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := mustParseSelect(t, tt.sql)

			ref, columns, ok := singleSourceSelect(stmt)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}

			if !tt.wantOK {
				return
			}

			if ref.Schema != tt.wantSchema {
				t.Errorf("schema = %q, want %q", ref.Schema, tt.wantSchema)
			}

			if ref.Name != tt.wantTable {
				t.Errorf("table = %q, want %q", ref.Name, tt.wantTable)
			}

			if len(columns) != tt.wantCols {
				t.Errorf("len(columns) = %d, want %d", len(columns), tt.wantCols)
			}
		})
	}
}

func Test_tableRefFullName(t *testing.T) {
	tests := []struct {
		name string
		ref  *ast.TableRef
		want string
	}{
		{name: "bare name", ref: &ast.TableRef{Name: "t"}, want: "t"},
		{name: "schema-qualified", ref: &ast.TableRef{Schema: "s", Name: "t"}, want: "s.t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tableRefFullName(tt.ref); got != tt.want {
				t.Errorf("tableRefFullName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_resultColumnCandidates(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		driverColumns []string
		want          []string
	}{
		{
			name:          "star expands to driver columns",
			sql:           `SELECT * FROM t`,
			driverColumns: []string{"id", "name"},
			want:          []string{"id", "name"},
		},
		{
			name:          "plain columns",
			sql:           `SELECT id, name FROM t`,
			driverColumns: []string{"id", "name"},
			want:          []string{"id", "name"},
		},
		{
			name:          "aliased column excluded",
			sql:           `SELECT id, name AS n FROM t`,
			driverColumns: []string{"id", "n"},
			want:          []string{"id"},
		},
		{
			name:          "star mixed with other columns is not expanded",
			sql:           `SELECT id, * FROM t`,
			driverColumns: []string{"id", "id", "name"},
			want:          []string{"id"},
		},
		{
			name:          "expression column excluded",
			sql:           `SELECT id, UPPER(name) FROM t`,
			driverColumns: []string{"id", "UPPER(name)"},
			want:          []string{"id"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt := mustParseSelect(t, tt.sql)

			_, columns, ok := singleSourceSelect(stmt)
			if !ok {
				t.Fatalf("singleSourceSelect() ok = false for %q", tt.sql)
			}

			got := resultColumnCandidates(columns, tt.driverColumns)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("resultColumnCandidates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_choosePrimaryKey(t *testing.T) {
	tests := []struct {
		name          string
		candidates    []string
		driverColumns []string
		pk            string
		unique        map[string]bool
		want          string
	}{
		{
			name:          "prefers primary key over other unique candidate",
			candidates:    []string{"email", "id"},
			driverColumns: []string{"email", "id"},
			pk:            "id",
			unique:        map[string]bool{"id": true, "email": true},
			want:          "id",
		},
		{
			name:          "falls back to first unique candidate when pk absent from select list",
			candidates:    []string{"email"},
			driverColumns: []string{"email"},
			pk:            "id",
			unique:        map[string]bool{"id": true, "email": true},
			want:          "email",
		},
		{
			name:          "no qualifying candidate",
			candidates:    []string{"name"},
			driverColumns: []string{"name"},
			pk:            "id",
			unique:        map[string]bool{"id": true},
			want:          "",
		},
		{
			name:          "case-insensitive match returns driver's own casing",
			candidates:    []string{"ID"},
			driverColumns: []string{"ID"},
			pk:            "id",
			unique:        map[string]bool{"id": true},
			want:          "ID",
		},
		{
			name:          "composite key column not offered as unique is never chosen",
			candidates:    []string{"a", "b"},
			driverColumns: []string{"a", "b"},
			pk:            "",
			unique:        map[string]bool{},
			want:          "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := choosePrimaryKey(tt.candidates, tt.driverColumns, tt.pk, tt.unique)
			if got != tt.want {
				t.Errorf("choosePrimaryKey() = %q, want %q", got, tt.want)
			}
		})
	}
}
