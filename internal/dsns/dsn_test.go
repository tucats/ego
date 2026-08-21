package dsns

import (
	"os"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/tucats/ego/internal/caches"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

func TestCacheError(t *testing.T) {
	t.Run("cache test", func(t *testing.T) {
		type Bogus struct {
			Name string
		}

		// Create a temporary name for this test
		fileName := "test-" + uuid.NewString() + ".db"

		// Define a DSN service for this path. The DSN
		// service will open the SQLite database.
		service, err := defineDSNService("sqlite://" + fileName)
		if err != nil {
			t.Fatal(err)
		}

		defer func() {
			service.Flush()

			// Close the underlying database connections before removing the
			// files on disk. Previously there was no Close() to call at all,
			// so the *sql.DB handles were simply abandoned for the OS to
			// clean up whenever the test binary exited.
			if err := service.Close(); err != nil {
				t.Errorf("service.Close() failed: %v", err)
			}

			os.Remove(fileName)

			// SQLite runs in WAL (Write-Ahead Log) mode here (see
			// internal/resources/pragmas.go's applyWriterPragmas), which keeps
			// two sidecar files alongside the main database file: "-wal" (the
			// not-yet-checkpointed write log) and "-shm" (a shared-memory index
			// into that log). Removing only fileName left these two behind on
			// every test run, since nothing else in this test's cleanup path
			// knows about them. Close() above should have let SQLite check-
			// point and clean these up on its own, but the explicit removal
			// here is a harmless backstop if it doesn't (os.Remove on a file
			// that no longer exists is not an error).
			os.Remove(fileName + "-wal")
			os.Remove(fileName + "-shm")
		}()

		// Write a DSN to the service.
		err = service.WriteDSN(0, "testuser", defs.DSN{
			Name:     "test",
			Provider: defs.DeprecatedSqliteProvider,
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
			provider: defs.PostgresProvider,
			want:     "postgres://tom:secret@localhost:5432/default?sslmode=disable",
		},
		{
			name:     "simple",
			provider: defs.PostgresProvider,
			want:     "postgres://localhost:5432/simple?sslmode=disable",
		},
		{
			name:     "simple with DB",
			db:       "default",
			provider: defs.PostgresProvider,
			want:     "postgres://localhost:5432/default?sslmode=disable",
		},
		{
			name:     "simple with DB, user",
			db:       "default",
			user:     "tom",
			provider: defs.PostgresProvider,
			want:     "postgres://tom@localhost:5432/default?sslmode=disable",
		},
		{
			name:     "simple with DB, port",
			db:       "default",
			port:     5555,
			user:     "tom",
			password: "secret",
			provider: defs.PostgresProvider,
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
			provider: defs.PostgresProvider,
			want:     "postgres://tom:secret@dbserver:5555/default",
		},
		{
			name:     "sqlite with DB",
			db:       "test.db",
			provider: defs.DeprecatedSqliteProvider,
			want:     "sqlite://test.db",
		},
		{
			name:     "sqlite with extraneous ignored settings",
			db:       "test.db",
			provider: defs.DeprecatedSqliteProvider,
			host:     "zorba",
			port:     666,
			user:     "fozzie",
			password: "bear",
			want:     "sqlite://test.db",
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
