package settings

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

type SettingsPersistence interface {
	Save() error
	Load(application, name string) error
	DeleteProfile(name string) error
	UseProfile(name string)
	Close()
}

var Persistence SettingsPersistence
var persistenceLock = &sync.Mutex{}

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
		Persistence, err = newFileSettingsPersistence(application, config)

		return err

	case sqliteType, sqlite3Type, postgresType:
		Persistence, err = newDatabaseSettingsPersistence(application, scheme, config)

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

	return Persistence.Load(application, name)
}

// Save uses the current persistence layer for settings to save the current configuration.
func Save() error {
	persistenceLock.Lock()
	defer persistenceLock.Unlock()

	if Persistence == nil {
		return errors.ErrPersistenceNotInitialized.In("Save")
	}

	return Persistence.Save()
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

	Persistence.UseProfile(name)
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
