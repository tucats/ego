package dsns

import (
	"os"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/tucats/ego/caches"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

func TestCacheError(t *testing.T) {
	t.Run("cache test", func(t *testing.T) {
		type Bogus struct {
			Name string
		}

		// Create a temporary name for this test
		fname := "test-" + uuid.NewString() + ".db"

		// Definme a DSN service for this path. The DSN
		// service will open the SQLite database.
		service, err := defineDSNService("sqlite3://" + fname)
		if err != nil {
			t.Fatal(err)
		}

		defer func() {
			service.Flush()
			os.Remove(fname)
		}()

		// Write a DSN to the service.
		err = service.WriteDSN(0, "testuser", defs.DSN{
			Name:     "test",
			Provider: "sqlite3",
			Database: "default",
		})
		if err != nil {
			t.Fatal(err)
		}

		// Intentionally damage the cache entry for the "test" DSN,
		// by creating an item in the cache of the same name but not
		// of type defs.DSN
		caches.Add(caches.DSNCache, "test", Bogus{Name: "bogus"})

		// Attempt to retrieve the item, which will still be in the cache.
		// This must return an error indicating an invalid cache item.
		_, err = service.ReadDSN(0, "testuser", "test", true)
		if err == nil {
			t.Fatalf("Expected error reading DSN, got none")
		}

		if !errors.Equal(err, errors.ErrInvalidCacheItem) {
			t.Log("Expected cache type error, got: " + err.Error())
		}
	})
}

func TestNewDSN(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		db         string
		host       string
		port       int
		user       string
		password   string
		restricted bool
		secured    bool
		want       string
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
			d := NewDSN(tt.name, tt.provider, tt.db, tt.user, tt.password, tt.host, tt.port, tt.restricted, tt.secured)
			if got, _ := Connection(d); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TestDSN() = %v, want %v", got, tt.want)
			}
		})
	}
}
