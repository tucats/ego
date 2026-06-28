package resources

import (
	"strings"
	"testing"
)

// TestInsertUpdateDeleteSQL verifies that table names are properly quoted in the
// INSERT, UPDATE, and DELETE generators (DB-8).
func TestInsertUpdateDeleteSQL(t *testing.T) {
	h := ResHandle{
		Table: "credentials",
		Columns: []Column{
			{SQLName: "id", SQLType: SQLStringType, Primary: true},
			{SQLName: "name", SQLType: SQLStringType},
		},
	}

	if got := h.insertSQL(); !strings.Contains(got, `"credentials"`) {
		t.Errorf("insertSQL() = %q; want table name to be double-quoted", got)
	}

	if got := h.updateSQL(); !strings.Contains(got, `"credentials"`) {
		t.Errorf("updateSQL() = %q; want table name to be double-quoted", got)
	}

	if got := h.deleteRowSQL(); !strings.Contains(got, `"credentials"`) {
		t.Errorf("deleteRowSQL() = %q; want table name to be double-quoted", got)
	}
}

// TestCreateTableSQL verifies that createTableSQL() produces correct SQL DDL.
func TestCreateTableSQL(t *testing.T) {
	tests := []struct {
		name    string
		handle  ResHandle
		checks  []string // substrings that must appear in the output
		nocheck []string // substrings that must NOT appear in the output
	}{
		{
			name: "single primary key column",
			handle: ResHandle{
				Table: "credentials",
				Columns: []Column{
					{SQLName: "id", SQLType: SQLStringType, Primary: true},
				},
			},
			checks:  []string{`create table "credentials"`, `"id" TEXT`, `primary key`},
			nocheck: []string{"nullable", "NULL"},
		},
		{
			name: "column marked as nullable emits NULL keyword",
			handle: ResHandle{
				Table: "mytable",
				Columns: []Column{
					{SQLName: "id", SQLType: SQLStringType, Primary: true},
					{SQLName: "note", SQLType: SQLStringType, Nullable: true},
				},
			},
			checks:  []string{`create table "mytable"`, `"note" TEXT NULL`},
			nocheck: []string{"nullable"},
		},
		{
			name: "column not marked nullable emits no nullability keyword",
			handle: ResHandle{
				Table: "mytable",
				Columns: []Column{
					{SQLName: "score", SQLType: SQLIntType},
				},
			},
			checks:  []string{`create table "mytable"`, `"score" integer`},
			nocheck: []string{"nullable", " NULL", "NOT NULL"},
		},
		{
			name: "table name is properly double-quoted",
			handle: ResHandle{
				Table: `my"table`,
				Columns: []Column{
					{SQLName: "id", SQLType: SQLStringType},
				},
			},
			checks:  []string{`create table "my""table"`},
			nocheck: []string{`"my\"table"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.handle.createTableSQL()

			for _, want := range tt.checks {
				if !strings.Contains(got, want) {
					t.Errorf("createTableSQL() = %q; want it to contain %q", got, want)
				}
			}

			for _, bad := range tt.nocheck {
				if strings.Contains(got, bad) {
					t.Errorf("createTableSQL() = %q; must NOT contain %q", got, bad)
				}
			}
		})
	}
}
