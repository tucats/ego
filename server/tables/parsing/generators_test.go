package parsing

import (
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/tucats/ego/defs"
)

func TestFormUpdateQuery(t *testing.T) {
	type args struct {
		urlString string
		items     map[string]any
		columns   []defs.DBColumn
		user      string
		provider  string
	}

	// Test cases
	tests := []struct {
		name       string
		args       args
		want       string
		wantValues []any
		wantErr    string
	}{
		{
			name: "simple update query with NULL constant value",
			args: args{
				urlString: "http://example.com/tables/data/rows?filter=EQ(id,.nil)",
				items:     map[string]any{"name": "John", "age": 30},
				columns: []defs.DBColumn{
					{
						Name: "name",
						Type: "string",
					},
					{
						Name:     "age",
						Type:     "int",
						Nullable: defs.BoolValue{Specified: true, Value: true},
					},
				},
				user:     "admin",
				provider: defs.SqliteProvider,
			},
			want:       `UPDATE "data" SET "age"=$1,"name"=$2 WHERE ("id" IS NULL )`,
			wantValues: []any{30, "John"},
			wantErr:    "",
		},
		{
			name: "simple update query of one field with bogus filter",
			args: args{
				urlString: "http://example.com/tables/data/rows?filter=FAUX(id,1)",
				items:     map[string]any{"owned_by": "John"},
				columns:   []defs.DBColumn{{Name: "owned_by", Type: "string"}},
				user:      "admin",
				provider:  defs.SqliteProvider,
			},
			wantErr: `unexpected token: Identifier "FAUX"`,
		},
		{
			name: "simple update query of one field",
			args: args{
				urlString: "http://example.com/tables/data/rows?filter=EQ(id,1)",
				items:     map[string]any{"owned_by": "John"},
				columns:   []defs.DBColumn{{Name: "owned_by", Type: "string"}},
				user:      "admin",
				provider:  defs.SqliteProvider,
			},
			want:       `UPDATE "data" SET "owned_by"=$1 WHERE ("id" = 1)`,
			wantValues: []any{"John"},
			wantErr:    "",
		},
		{
			name: "simple update query of two fields",
			args: args{
				urlString: "http://example.com/tables/data/rows?filter=EQ(id,1)",
				items:     map[string]any{"name": "John", "age": 30},
				columns:   []defs.DBColumn{{Name: "name", Type: "string"}, {Name: "age", Type: "int"}},
				user:      "admin",
				provider:  defs.SqliteProvider,
			},
			want:       `UPDATE "data" SET "age"=$1,"name"=$2 WHERE ("id" = 1)`,
			wantValues: []any{30, "John"},
			wantErr:    "",
		},
		{
			name: "update query of two fields with data type conversion of row data",
			args: args{
				urlString: "http://example.com/tables/data/rows?filter=EQ(id,1)",
				items:     map[string]any{"name": "John", "age": 30.0},
				columns:   []defs.DBColumn{{Name: "name", Type: "string"}, {Name: "age", Type: "int"}},
				user:      "admin",
				provider:  defs.SqliteProvider,
			},
			want:       `UPDATE "data" SET "age"=$1,"name"=$2 WHERE ("id" = 1)`,
			wantValues: []any{30, "John"},
			wantErr:    "",
		},
		{
			name: "update query with multiple filters",
			args: args{
				urlString: `http://example.com/tables/data/rows?filter=AND(EQ(name,"John"),EQ(id,1))`,
				items:     map[string]any{"name": "John", "age": 30},
				columns:   []defs.DBColumn{{Name: "name", Type: "string"}, {Name: "age", Type: "int"}},
				user:      "admin",
				provider:  defs.SqliteProvider,
			},
			want:       `UPDATE "data" SET "age"=$1,"name"=$2 WHERE (("name" = 'John')  AND  ("id" = 1))`,
			wantValues: []any{30, "John"},
			wantErr:    "",
		},
	}

	for _, tt := range tests {
		expectedQuery := tt.want

		u, _ := url.Parse(tt.args.urlString)

		query, values, err := FormUpdateQuery(u, tt.args.user, tt.args.provider, tt.args.columns, tt.args.items)

		errorMessage := ""
		if err != nil {
			errorMessage = err.Error()
		}

		if errorMessage != tt.wantErr {
			t.Errorf("%s, Unexpected error: %v", tt.name, err)

			continue
		}

		if query != expectedQuery {
			t.Errorf("%s, Unexpected query. Expected: %s, Got: %s", tt.name, expectedQuery, query)
		}

		if !reflect.DeepEqual(values, tt.wantValues) {
			t.Errorf("%s, Unexpected values. Expected: %v, Got: %v", tt.name, tt.wantValues, values)
		}
	}
}

func TestFormSelectorDeleteQuery(t *testing.T) {
	type args struct {
		urlString string
		filter    []string
		columns   string
		table     string
		user      string
		verb      string
		provider  string
	}

	// Test cases
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr string
	}{
		{
			name: "invalid query filter operator",
			args: args{
				urlString: "http://example.com",
				filter:    []string{`FAUX("age", 30)`},
				columns:   "id, name, age",
				table:     "users",
				user:      "admin",
				verb:      "SELECT",
				provider:  defs.SqliteProvider,
			},
			wantErr: `unexpected token: Identifier "FAUX"`,
		},
		{
			name: "invalid query filter term list",
			args: args{
				urlString: "http://example.com",
				filter:    []string{`EQ("age", )`},
				columns:   "id, name, age",
				table:     "users",
				user:      "admin",
				verb:      "SELECT",
				provider:  defs.SqliteProvider,
			},
			wantErr: "missing parenthesis",
		},
		{
			name: "simple query",
			args: args{
				urlString: "http://example.com",
				filter:    []string{},
				columns:   "id, name, age",
				table:     "users",
				user:      "admin",
				verb:      "SELECT",
				provider:  defs.SqliteProvider,
			},
			want:    `SELECT "id","name","age" FROM "users"  LIMIT 1000`,
			wantErr: "",
		},
		{
			name: "query with one sort column",
			args: args{
				urlString: "http://example.com/tables/data?order=age",
				filter:    []string{},
				columns:   "",
				table:     "users",
				user:      "admin",
				verb:      "SELECT",
				provider:  defs.SqliteProvider,
			},
			want:    `SELECT * FROM "users" ORDER BY age  LIMIT 1000`,
			wantErr: "",
		},
		{
			name: "query with two sort columns",
			args: args{
				urlString: "http://example.com/tables/data?order=age,name",
				filter:    []string{},
				columns:   "",
				table:     "users",
				user:      "admin",
				verb:      "SELECT",
				provider:  defs.SqliteProvider,
			},
			want:    `SELECT * FROM "users" ORDER BY age,name  LIMIT 1000`,
			wantErr: "",
		},
		{
			name: "query with one filter",
			args: args{
				urlString: "http://example.com",
				filter:    []string{"EQ(name,\"John\")"},
				columns:   "id, name, age",
				table:     "users",
				user:      "admin",
				verb:      "DELETE",
				provider:  defs.SqliteProvider,
			},
			want:    `DELETE FROM "users" WHERE ("name" = 'John')`,
			wantErr: "",
		},
		{
			name: "query with two filters",
			args: args{
				urlString: "http://example.com",
				filter:    []string{"EQ(name,\"John\")", "GT(age, 30)"},
				columns:   "id, name, age",
				table:     "users",
				user:      "admin",
				verb:      "DELETE",
				provider:  defs.SqliteProvider,
			},
			want:    `DELETE FROM "users" WHERE ("name" = 'John') AND ("age" > 30)`,
			wantErr: "",
		},
		{
			name: "query with two filters and one sort column",
			args: args{
				urlString: "http://example.com/tables/data?order=name",
				filter:    []string{"EQ(name,\"John\")", "GT(age, 30)"},
				columns:   "id, name, age",
				table:     "users",
				user:      "admin",
				verb:      "SELECT",
				provider:  defs.SqliteProvider,
			},
			want:    `SELECT "id","name","age" FROM "users" WHERE ("name" = 'John') AND ("age" > 30) ORDER BY name  LIMIT 1000`,
			wantErr: "",
		},
	}

	for _, tt := range tests {
		expectedQuery := tt.want

		u, _ := url.Parse(tt.args.urlString)

		query, err := FormSelectorDeleteQuery(u,
			tt.args.filter,
			tt.args.columns,
			tt.args.table,
			tt.args.user,
			tt.args.verb,
			tt.args.provider)

		errMessage := ""
		if err != nil {
			errMessage = err.Error()
		}

		if errMessage != tt.wantErr {
			t.Errorf("%s, Unexpected error: %v", tt.name, err)

			continue
		}

		if query != expectedQuery {
			t.Errorf("%s, Unexpected query. Expected: %s, Got: %s", tt.name, expectedQuery, query)
		}
	}
}

// TestCoerceToColumnType_TimeTypes verifies that CoerceToColumnType correctly converts
// date/time values for all recognised column type name variants.
//
// The function must:
//   - Parse RFC 3339 strings into time.Time for "timestamp", "time", "date", and their aliases.
//   - Pass an already-typed time.Time through unchanged (PostgreSQL read path).
//   - Produce a zero time.Time{} when the input value is nil.
//   - Leave non-time columns unaffected.
func TestCoerceToColumnType_TimeTypes(t *testing.T) {
	// A fixed reference time used throughout the tests.
	// time.Date returns a time.Time in the given location; UTC is used here so
	// the expected value is unambiguous regardless of the machine's local timezone.
	ref := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	// refRFC3339 is the RFC 3339 string representation of ref, which is what a
	// REST client typically sends in a JSON payload.
	refRFC3339 := "2024-06-15T12:00:00Z"

	tests := []struct {
		name       string
		columnType string // the Type field of DBColumn
		input      any    // the value arriving from the JSON payload (or nil)
		wantTime   bool   // true  → result must be a time.Time equal to ref
		wantZero   bool   // true  → result must be time.Time{} (zero value)
		wantErr    bool   // true  → the call must return a non-nil error
	}{
		// --- "timestamp" (portable lowercase name produced by getColumnInfo) ---
		{
			name:       "timestamp column: RFC 3339 string",
			columnType: "timestamp",
			input:      refRFC3339,
			wantTime:   true,
		},
		{
			name:       "timestamp column: already time.Time (PostgreSQL driver path)",
			columnType: "timestamp",
			input:      ref,
			wantTime:   true,
		},
		{
			name:       "timestamp column: nil produces zero time.Time",
			columnType: "timestamp",
			input:      nil,
			wantZero:   true,
		},

		// --- "timestamptz" (raw PostgreSQL type name that may appear before normalisation) ---
		{
			name:       "timestamptz column: RFC 3339 string",
			columnType: "timestamptz",
			input:      refRFC3339,
			wantTime:   true,
		},

		// --- "timestamp with time zone" (raw PostgreSQL DDL name) ---
		{
			name:       "timestamp with time zone: RFC 3339 string",
			columnType: "timestamp with time zone",
			input:      refRFC3339,
			wantTime:   true,
		},

		// --- "time" (portable name) ---
		{
			name:       "time column: RFC 3339 string",
			columnType: "time",
			input:      refRFC3339,
			wantTime:   true,
		},

		// --- "date" (portable name) ---
		{
			name:       "date column: RFC 3339 string",
			columnType: "date",
			input:      refRFC3339,
			wantTime:   true,
		},

		// --- "datetime" (legacy alias preserved from the original implementation) ---
		{
			name:       "datetime column: RFC 3339 string",
			columnType: "datetime",
			input:      refRFC3339,
			wantTime:   true,
		},

		// --- non-time columns must be unaffected by the new time logic ---
		{
			name:       "string column: value passes through as string",
			columnType: "string",
			input:      "hello",
			// wantTime and wantZero are both false; we check the type separately below.
		},
	}

	for _, tt := range tests {
		// Build a minimal columns slice containing just the column under test.
		columns := []defs.DBColumn{
			{Name: "col", Type: tt.columnType},
		}

		got, err := CoerceToColumnType("col", tt.input, columns)

		// Check the error expectation first.
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: error = %v, wantErr %v", tt.name, err, tt.wantErr)

			continue
		}

		if tt.wantErr {
			continue // nothing else to check
		}

		if tt.wantZero {
			// Expect a zero time.Time{}.
			z, ok := got.(time.Time)
			if !ok {
				t.Errorf("%s: expected time.Time{}, got %T", tt.name, got)

				continue
			}

			if !z.IsZero() {
				t.Errorf("%s: expected zero time.Time, got %v", tt.name, z)
			}

			continue
		}

		if tt.wantTime {
			// Expect a time.Time equal to ref (after UTC normalisation).
			gotTime, ok := got.(time.Time)
			if !ok {
				t.Errorf("%s: expected time.Time, got %T (%v)", tt.name, got, got)

				continue
			}

			// Compare as UTC to avoid false failures due to timezone representation.
			if !gotTime.UTC().Equal(ref.UTC()) {
				t.Errorf("%s: expected %v, got %v", tt.name, ref.UTC(), gotTime.UTC())
			}

			continue
		}
	}
}

// TestCoerceToColumnType_UnknownColumn verifies that CoerceToColumnType returns an error
// when asked to coerce a column name that does not exist in the metadata.
//
// The row-ID pseudo-column ("_row_id_") is exempt from this check because it is managed
// internally and is not declared in the user's column list.
func TestCoerceToColumnType_UnknownColumn(t *testing.T) {
	columns := []defs.DBColumn{
		{Name: "name", Type: "string"},
	}

	// An unknown column name should produce an error.
	if _, err := CoerceToColumnType("bogus", "x", columns); err == nil {
		t.Error("expected an error for unknown column, got nil")
	}

	// The row ID pseudo-column must not produce an error even though it is absent
	// from the columns slice.
	if _, err := CoerceToColumnType(defs.RowIDName, "abc", columns); err != nil {
		t.Errorf("unexpected error for row-ID column: %v", err)
	}
}

// TestBindTimeValue verifies that bindTimeValue formats a time.Time correctly for each
// database provider, and leaves non-time values unchanged.
//
// The function is the final step in the write path before values are passed to
// db.Exec / db.Query.  Getting the format right is critical:
//   - SQLite stores dates as TEXT, so the driver must receive an RFC 3339 string.
//   - PostgreSQL's lib/pq driver needs a native time.Time to bind to TIMESTAMP columns.
func TestBindTimeValue(t *testing.T) {
	// A fixed reference time in UTC.  Using a fixed value avoids flaky tests caused
	// by clock reads differing between the test and the assertion.
	ref := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		input    any
		provider string
		wantStr  string    // non-empty → expect this exact string back
		wantTime time.Time // non-zero → expect a time.Time equal to this value
		wantSame bool      // true → expect the returned value to be identical to input
	}{
		{
			name:     "SQLite: time.Time is formatted as UTC RFC 3339 string",
			input:    ref,
			provider: defs.SqliteProvider,
			wantStr:  "2024-06-15T12:00:00Z",
		},
		{
			name:     "SQLite deprecated alias: same behaviour as sqlite",
			input:    ref,
			provider: defs.DeprecatedSqliteProvider,
			wantStr:  "2024-06-15T12:00:00Z",
		},
		{
			name:     "PostgreSQL: time.Time is passed through as-is",
			input:    ref,
			provider: defs.PostgresProvider,
			wantTime: ref,
		},
		{
			name:     "non-time value: returned unchanged regardless of provider",
			input:    "hello",
			provider: defs.SqliteProvider,
			wantSame: true,
		},
		{
			name:     "nil value: returned unchanged",
			input:    nil,
			provider: defs.SqliteProvider,
			wantSame: true,
		},
	}

	for _, tt := range tests {
		got := bindTimeValue(tt.input, tt.provider)

		switch {
		case tt.wantStr != "":
			// Expect a specific string.
			s, ok := got.(string)
			if !ok {
				t.Errorf("%s: expected string, got %T (%v)", tt.name, got, got)

				continue
			}

			if s != tt.wantStr {
				t.Errorf("%s: expected %q, got %q", tt.name, tt.wantStr, s)
			}

		case !tt.wantTime.IsZero():
			// Expect a time.Time equal to the reference.
			gotTime, ok := got.(time.Time)
			if !ok {
				t.Errorf("%s: expected time.Time, got %T (%v)", tt.name, got, got)
				
				continue
			}

			if !gotTime.UTC().Equal(tt.wantTime.UTC()) {
				t.Errorf("%s: expected %v, got %v", tt.name, tt.wantTime, gotTime)
			}

		case tt.wantSame:
			// Expect the same value (by equality) to be returned.
			if !reflect.DeepEqual(got, tt.input) {
				t.Errorf("%s: expected %v, got %v", tt.name, tt.input, got)
			}
		}
	}
}

// TestFormInsertQuery_TimeColumns verifies that FormInsertQuery produces the correct
// parameter value for timestamp/time/date columns on both SQLite and PostgreSQL.
//
// For SQLite the value bound to $1 must be an RFC 3339 string (because SQLite stores
// dates as TEXT).  For PostgreSQL the value must be a native time.Time (because lib/pq
// binds it directly to a TIMESTAMP WITH TIME ZONE column).
func TestFormInsertQuery_TimeColumns(t *testing.T) {
	// The reference time as an RFC 3339 string — this is what a REST client sends.
	inputStr := "2024-06-15T12:00:00Z"

	// The expected time.Time after parsing.
	expectedTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	columns := []defs.DBColumn{
		{Name: "event_time", Type: "timestamp"},
	}

	items := map[string]any{
		"event_time": inputStr,
	}

	// --- SQLite path ---
	_, sqliteValues, err := FormInsertQuery("events", "admin", defs.SqliteProvider, columns, items)
	if err != nil {
		t.Fatalf("SQLite FormInsertQuery error: %v", err)
	}

	if len(sqliteValues) != 1 {
		t.Fatalf("SQLite: expected 1 value, got %d", len(sqliteValues))
	}

	// For SQLite the bound value must be an RFC 3339 string.
	sqliteStr, ok := sqliteValues[0].(string)
	if !ok {
		t.Errorf("SQLite: expected string value, got %T (%v)", sqliteValues[0], sqliteValues[0])
	} else if sqliteStr != "2024-06-15T12:00:00Z" {
		t.Errorf("SQLite: expected RFC 3339 string %q, got %q", "2024-06-15T12:00:00Z", sqliteStr)
	}

	// --- PostgreSQL path ---
	_, pgValues, err := FormInsertQuery("events", "admin", defs.PostgresProvider, columns, items)
	if err != nil {
		t.Fatalf("PostgreSQL FormInsertQuery error: %v", err)
	}

	if len(pgValues) != 1 {
		t.Fatalf("PostgreSQL: expected 1 value, got %d", len(pgValues))
	}

	// For PostgreSQL the bound value must be a native time.Time.
	pgTime, ok := pgValues[0].(time.Time)
	if !ok {
		t.Errorf("PostgreSQL: expected time.Time value, got %T (%v)", pgValues[0], pgValues[0])
	} else if !pgTime.UTC().Equal(expectedTime) {
		t.Errorf("PostgreSQL: expected %v, got %v", expectedTime, pgTime)
	}
}

// TestFormUpdateQuery_TimeColumns verifies that FormUpdateQuery binds time column values
// correctly for both SQLite and PostgreSQL, mirroring TestFormInsertQuery_TimeColumns.
func TestFormUpdateQuery_TimeColumns(t *testing.T) {
	inputStr := "2024-06-15T12:00:00Z"
	expectedTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

	columns := []defs.DBColumn{
		{Name: "event_time", Type: "timestamp"},
	}

	items := map[string]any{
		"event_time": inputStr,
	}

	// --- SQLite path ---
	_, sqliteValues, err := FormUpdateQuery(mustParseURL("http://host/tables/events/rows?filter=EQ(id,1)"),
		"admin", defs.SqliteProvider, columns, items)
	if err != nil {
		t.Fatalf("SQLite FormUpdateQuery error: %v", err)
	}

	if len(sqliteValues) == 0 {
		t.Fatal("SQLite: expected at least one value")
	}

	// The first value corresponds to the SET clause.
	sqliteStr, ok := sqliteValues[0].(string)
	if !ok {
		t.Errorf("SQLite: expected string, got %T (%v)", sqliteValues[0], sqliteValues[0])
	} else if sqliteStr != "2024-06-15T12:00:00Z" {
		t.Errorf("SQLite: expected RFC 3339 string, got %q", sqliteStr)
	}

	// --- PostgreSQL path ---
	_, pgValues, err := FormUpdateQuery(mustParseURL("http://host/tables/events/rows?filter=EQ(id,1)"),
		"admin", defs.PostgresProvider, columns, items)
	if err != nil {
		t.Fatalf("PostgreSQL FormUpdateQuery error: %v", err)
	}

	if len(pgValues) == 0 {
		t.Fatal("PostgreSQL: expected at least one value")
	}

	pgTime, ok := pgValues[0].(time.Time)
	if !ok {
		t.Errorf("PostgreSQL: expected time.Time, got %T (%v)", pgValues[0], pgValues[0])
	} else if !pgTime.UTC().Equal(expectedTime) {
		t.Errorf("PostgreSQL: expected %v, got %v", expectedTime, pgTime)
	}
}

// mustParseURL is a test helper that parses a URL string and panics on failure.
// It avoids repetitive error-checking boilerplate in table-driven tests where the
// URL strings are hard-coded constants that are always valid.
func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}

	return u
}
