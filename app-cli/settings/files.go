// Package settings manages the persistent user profile used by the command
// application infrastructure. This includes automatically reading any
// profile in as part of startup, and of updating the profile as needed.
package settings

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
)

// ProfileDirectory is the name of the invisible directory that is created
// in the user's home directory to host configuration data. This is the
// default value, but the main program can override it before starting the
// app to choose a different directory name.
var ProfileDirectory = ".org.fernwood"

// Default permission for the ProfileDirectory and the ProfileFile.
const securePermission = 0700

// Configuration version number. This is used to ensure that the
// configuration file is compatible with the current version of the
// application. The default is zero.
const ConfigurationVersion = 0

// DefaultConfiguration is a localized string that contains the
// local text for "Default configuration".
var DefaultConfiguration = i18n.L("Default.configuration")

// ProfileFile is the name of the configuration file that contains the
// profiles.
var ProfileFile = "config.json"

// ProfileName is the name of the configuration being used. The default
// configuration is always named "default".
var ProfileName = "default"

// Configuration describes what is known about a configuration.
type Configuration struct {
	// A textual description of the configuration. This is displayed
	// when the profiles are listed.
	Description string `json:"description,omitempty"`

	// A UUID expressed as a string, which uniquely identifies this
	// configuration for the life of the application.
	ID string `json:"id,omitempty"`

	// The date and time of the last modification of this profile,
	// expressed as a string.
	Modified string `json:"modified,omitempty"`

	// The version of the configuration file format. This is used to
	// ensure that the file is compatible with the current version of
	// the application. The default is zero.

	Version int `json:"version"`

	// The Items map contains the individual configuration values. Each
	// has a key which is the name of the option, and a string value for
	// that configuration item. Configuration items that are not strings
	// must be serialized as a string.
	Items map[string]string `json:"items"`
}

// CurrentConfiguration describes the current configuration that is active.
var CurrentConfiguration *Configuration

// Load reads in the named profile, if it exists.
func Load(application string, name string) error {
	var c = Configuration{
		Description: DefaultConfiguration,
		Version:     ConfigurationVersion,
		Items:       map[string]string{},
	}

	CurrentConfiguration = &c
	Configurations = map[string]*Configuration{"default": CurrentConfiguration}
	ProfileFile = application + ".json"

	home, err := os.UserHomeDir()
	if err != nil {
		return errors.New(err)
	}

	path := filepath.Join(home, ProfileDirectory, ProfileFile)

	configFile, err := os.Open(path)
	if err != nil {
		return errors.New(err)
	}

	defer configFile.Close()
	// read our opened jsonFile as a byte array.
	byteValue, _ := io.ReadAll(configFile)

	// we unmarshal our byteArray which contains our
	// jsonFile's content into the config map which we defined above
	err = json.Unmarshal(byteValue, &Configurations)
	if err == nil {
		if name == "" {
			name = ProfileName
		}

		c, found := Configurations[name]

		if !found {
			c = &Configuration{
				Description: DefaultConfiguration,
				Version:     ConfigurationVersion,
				Items:       map[string]string{}}
			Configurations[name] = c
			ProfileDirty = true
		}

		ProfileName = name
		CurrentConfiguration = c
	}

	if err != nil {
		err = errors.New(err)
	}

	return err
}

// Save the current configuration to persistent disk storage.
func Save() error {
	// So we even need to do anything?
	if !ProfileDirty {
		return nil
	}

	// Does the directory exist?
	home, err := os.UserHomeDir()
	if err != nil {
		return errors.New(err)
	}

	path := filepath.Join(home, ProfileDirectory)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		_ = os.MkdirAll(path, securePermission)
	}

	path = filepath.Join(path, ProfileFile)

	// Make sure every configuration has an id
	for n := range Configurations {
		c := Configurations[n]
		if c.ID == "" {
			c.ID = uuid.New().String()
			Configurations[n] = c

			ui.Log(ui.AppLogger, "Creating configuration \"%s\" with id %s", n, c.ID)
		}
	}

	byteBuffer, _ := json.MarshalIndent(&Configurations, "", "  ")
	err = os.WriteFile(path, byteBuffer, securePermission)

	if err != nil {
		err = errors.New(err)
	}

	return err
}

// UseProfile specifies the name of the profile to use, if other
// than the default.
func UseProfile(name string) {
	c, found := Configurations[name]
	if !found {
		c = &Configuration{
			Description: name + " " + i18n.L("configuration"),
			Version:     ConfigurationVersion,
			Items:       map[string]string{}}
		Configurations[name] = c
		ProfileDirty = true
	}

	ProfileName = name
	CurrentConfiguration = c
}

// DeleteProfile deletes an entire named configuration.
func DeleteProfile(key string) error {
	if c, ok := Configurations[key]; ok {
		if c.ID == getCurrentConfiguration().ID {
			ui.Log(ui.AppLogger, "cannot delete active profile")

			return errors.ErrCannotDeleteActiveProfile.Context(key)
		}

		delete(Configurations, key)

		c.Modified = time.Now().Format(time.RFC1123Z)

		ProfileDirty = true

		err := Save()
		if err == nil {
			ui.Log(ui.AppLogger, "deleted profile %s (%s)", key, c.ID)
		}

		return err
	}

	ui.Log(ui.AppLogger, "no such profile to delete: %s", key)

	return errors.ErrNoSuchProfile.Context(key)
}

func getCurrentConfiguration() *Configuration {
	if CurrentConfiguration == nil {
		CurrentConfiguration = &Configuration{
			Description: DefaultConfiguration,
			Version:     ConfigurationVersion,
			Items:       map[string]string{}}
	}

	return CurrentConfiguration
}
