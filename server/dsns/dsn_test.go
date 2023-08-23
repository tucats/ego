package dsns

import (
	"reflect"
	"testing"
)

func TestNewDSN(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		db       string
		host     string
		port     int
		user     string
		password string
		native   bool
		secured  bool
		want     string
	}{
		{
			name:     "simple with DB, user, pw",
			db:       "default",
			user:     "tom",
			password: "secret",
			provider: "postgres",
			want:     "postgres://tom:secret@localhost:5432/default?sslmode=disable",
		},
		{
			name:     "simple",
			provider: "postgres",
			want:     "postgres://localhost:5432/simple?sslmode=disable",
		},
		{
			name:     "simple with DB",
			db:       "default",
			provider: "postgres",
			want:     "postgres://localhost:5432/default?sslmode=disable",
		},
		{
			name:     "simple with DB, user",
			db:       "default",
			user:     "tom",
			provider: "postgres",
			want:     "postgres://tom@localhost:5432/default?sslmode=disable",
		},
		{
			name:     "simple with DB, port",
			db:       "default",
			port:     5555,
			user:     "tom",
			password: "secret",
			provider: "postgres",
			want:     "postgres://tom:secret@localhost:5555/default?sslmode=disable",
		},
		{
			name:     "simple with DB, host, port, secured",
			db:       "default",
			host:     "dbserver",
			secured:  true,
			port:     5555,
			user:     "tom",
			password: "secret",
			provider: "postgres",
			want:     "postgres://tom:secret@dbserver:5555/default",
		},
		{
			name:     "sqlite3 with DB",
			db:       "test.db",
			provider: "sqlite3",
			want:     "sqlite3://test.db",
		},
		{
			name:     "sqlite3 with extraneous ignored settings",
			db:       "test.db",
			provider: "sqlite3",
			host:     "zorba",
			port:     666,
			user:     "fozzie",
			password: "bear",
			want:     "sqlite3://test.db",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDSN(tt.name, tt.provider, tt.db, tt.user, tt.password, tt.host, tt.port, tt.native, tt.secured)
			if got, _ := Connection(d); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TestDSN() = %v, want %v", got, tt.want)
			}
		})
	}
}
