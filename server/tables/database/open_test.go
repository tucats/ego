package database

import "testing"

func TestMasked_InvalidURL(t *testing.T) {
	input := "invalid_url"
	expected := input

	result := redactURLString(input)

	if result != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, result)
	}
}

func TestMasked_SensitiveInfoInPath(t *testing.T) {
	input := "postgres://user:password@localhost/dbname/sensitive_info"
	expected := "postgres://user:xxxxx@localhost/dbname/sensitive_info"

	result := redactURLString(input)

	if result != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, result)
	}
}

func TestMasked_Sqlite(t *testing.T) {
	input := "sqlite3://foo.db"
	expected := "sqlite3://foo.db"

	result := redactURLString(input)

	if result != expected {
		t.Errorf("Expected '%s', but got '%s'", expected, result)
	}
}
