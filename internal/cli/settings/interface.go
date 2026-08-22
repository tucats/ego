package settings

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
)

type SettingsPersistence interface {
	Save(config *Configuration) error
	Load(application, name string) (*Configuration, error)
	DeleteProfile(name string) error
	UseProfile(name string) (*Configuration, error)
	Close()
}

var Persistence SettingsPersistence
var persistenceLock = &sync.Mutex{}

// SkipEncryptedProfileValues, when true, tells Load to skip decrypting the
// handful of profile settings that are stored encrypted at rest --
// ego.logon.token, ego.logon.refresh.token, ego.server.token.key,
// ego.server.database.credentials, ego.server.database.url, and
// ego.server.default.credential (see encryptedKeyValue in defs.go). Both
// persistence backends decrypt every one of these on every Load(), whether
// or not the invocation ever reads them: the file-backed persistence
// (files.go) does it in readOutboardConfigFiles, and the database-backed
// persistence (databases.go) does it inline while scanning the settings
// table. Each decryption derives its key via Argon2id (32 MiB memory cost,
// see the argon2* constants in crypto.go), a deliberately expensive
// operation that measured at ~40-50ms per call in local testing -- so a
// profile with two or three of these values present can spend upwards of
// 100ms on decryption alone before Load() even returns.
//
// This is set (never by end users -- there is no CLI flag or profile
// setting for it) by internal/cli/app/run.go, immediately before it calls
// Load, when it detects the invocation is a child-service process spawned
// by internal/server/services/child.go (`ego --service <file|pipe>`). A
// child-service process runs exactly one already-authorized REST request
// and exits; it never calls settings.Get on any of the six keys above --
// the one value a service might need from that set, the DSN database URL,
// is instead handed to it explicitly in the per-request JSON payload (see
// ChildServiceRequest.DSNDatabaseURL), which the parent server already
// decrypted once when it first loaded its own profile. Skipping the
// decryption here removes what was, before this change, the single
// largest cost in a child-service request's startup latency, with no
// change in behavior on that path since the values were never used there.
var SkipEncryptedProfileValues = false

// Initialize creates the correct instance of settings persistence based on the
// provided configuration.
func Initialize(application, config string) error {
	var err error

	if e := os.Getenv("EGO_CONFIG"); e != "" {
		config = e
	}

	scheme := "file"

	if pos := strings.Index(config, "://"); pos >= 0 {
		scheme = config[:pos]
		config = config[pos+3:]
	}

	// If the prefix has a tilde reference to the user's home directory, resolve it now.
	if strings.HasPrefix(config, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		config = filepath.Join(home, strings.TrimPrefix(config, "~/"))
	}

	ui.Log(ui.AppLogger, "settings.initialize", ui.A{
		"application": application,
		"config":      config,
		"scheme":      scheme})

	switch scheme {
	case fileType:
		Persistence, err = NewFileConfigService(application, config)

		return err

	case defs.SqliteProvider, defs.DeprecatedSqliteProvider, defs.PostgresProvider:
		Persistence, err = NewDatabaseConfigService(application, scheme, config)

		return err

	case configType:
		return errors.ErrUnsupportedSettingsScheme.Context(scheme)

	default:
		return errors.ErrInvalidSettingsScheme.Context(scheme)
	}
}

// Load uses the current persistence layer for settings to load a configuration.
func Load(application, name string) error {
	persistenceLock.Lock()
	defer persistenceLock.Unlock()

	if Persistence == nil {
		if err := Initialize(application, name); err != nil {
			return err
		}
	}

	c, err := Persistence.Load(application, name)
	if err == nil {
		CurrentConfiguration = c
	}

	return err
}

// Save uses the current persistence layer for settings to save the current configuration.
func Save() error {
	persistenceLock.Lock()
	defer persistenceLock.Unlock()

	if Persistence == nil {
		return errors.ErrPersistenceNotInitialized.In("Save")
	}

	return Persistence.Save(CurrentConfiguration)
}

// DeleteProfile uses the current persistence layer for settings to delete a configuration.
func DeleteProfile(name string) error {
	persistenceLock.Lock()
	defer persistenceLock.Unlock()

	if Persistence == nil {
		return errors.ErrPersistenceNotInitialized.In("DeleteProfile")
	}

	return Persistence.DeleteProfile(name)
}

// UseProfile uses the current persistence layer for settings to use a specific configuration.
func UseProfile(name string) {
	persistenceLock.Lock()
	defer persistenceLock.Unlock()

	if Persistence == nil {
		return
	}

	c, _ := Persistence.UseProfile(name)

	CurrentConfiguration = c
}

// Close out database operations when the application exits.
func Close() {
	persistenceLock.Lock()
	defer persistenceLock.Unlock()

	if Persistence == nil {
		return
	}

	Persistence.Close()
}
