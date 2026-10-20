package tables

import (
	"net/http"
	"testing"
)

// TestSQLTransaction_SQLiteTimestampIsRFC3339 guards against @sql formatting a
// SQLite TIMESTAMP column differently from the /rows endpoint. The SQLite
// driver reports ScanType() "string" for a TEXT-stored timestamp on a live
// result set, which used to classify the column as a string and render the
// driver's time.Time value via data.String() as RFC 822 ("19 Oct 26 23:58
// +0000") instead of marshaling it as RFC 3339.
func TestSQLTransaction_SQLiteTimestampIsRFC3339(t *testing.T) {
	post := setUpSQLPkeyTest(t)

	const want = "2026-10-19T23:58:34Z"

	status, rowSet := post([]string{
		`CREATE TABLE times (t TIMESTAMP)`,
		`INSERT INTO times (t) VALUES ('` + want + `')`,
		`SELECT * FROM times`,
	})

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if len(rowSet.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rowSet.Rows))
	}

	if got := rowSet.Rows[0]["t"]; got != want {
		t.Errorf("t = %v, want %q", got, want)
	}
}
