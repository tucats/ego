package tables

import (
	"reflect"
	"testing"
)

func TestNewDSN(t *testing.T) {
	tests := []struct {
		name     string
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
			name: "simple",
			want: "postgres://localhost:5432/simple?sslmode=disable",
		},
		{
			name: "simple with DB",
			db:   "default",
			want: "postgres://localhost:5432/default?sslmode=disable",
		},
		{
			name: "simple with DB, user",
			db:   "default",
			user: "tom",
			want: "postgres://tom@localhost:5432/default?sslmode=disable",
		},
		{
			name:     "simple with DB, user, pw",
			db:       "default",
			user:     "tom",
			password: "secret",
			want:     "postgres://tom:secret@localhost:5432/default?sslmode=disable",
		},
		{
			name:     "simple with DB, port",
			db:       "default",
			port:     5555,
			user:     "tom",
			password: "secret",
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
			want:     "postgres://tom:secret@dbserver:5555/default",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDSN(tt.name, tt.native)
			if tt.db != "" {
				d.DB(tt.db)
			}

			if tt.user != "" {
				d.User(tt.user, tt.password)
			}

			if tt.host != "" || tt.port != 0 {
				d.Address(tt.host, tt.port, tt.secured)
			}

			if got, _ := d.Connection(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TestDSN() = %v, want %v", got, tt.want)
			}
		})
	}
}
