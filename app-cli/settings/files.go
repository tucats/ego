// Package settings manages the persistent user profile used by the command
// application infrastructure. This includes automatically reading any
// profile in as part of startup, and of updating the profile as needed.
package settings

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	// The name of the configuration. This is the name that is used to
	// select the configuration to use.
	Name string `json:"name"`

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

	// Flag indicating if this is a modified configuration.
	Dirty bool `json:"updated,omitempty"`

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
		Name:        name,
		Dirty:       true,
		Items:       map[string]string{},
	}

	ui.Log(ui.AppLogger, "Make configuration \"%s\" active", name)

	CurrentConfiguration = &c
	Configurations = map[string]*Configuration{"default": CurrentConfiguration}
	ProfileFile = application + ".json"

	home, err := os.UserHomeDir()
	if err != nil {
		return errors.New(err)
	}

	// First, make sure the profile directory exists.
	path := filepath.Join(home, ProfileDirectory)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		_ = os.MkdirAll(path, securePermission)
		ui.Log(ui.AppLogger, "Create configuration directory %s", path)
	}

	// If it's found, also read the old grouped configurations from the profile file
	path = filepath.Join(home, ProfileDirectory, ProfileFile)

	configFile, err := os.Open(path)
	if err == nil {
		defer configFile.Close()
		ui.Log(ui.AppLogger, "Reading combined configuration file \"%s\"", path)

		// read our opened jsonFile as a byte array.
		byteValue, _ := io.ReadAll(configFile)

		// we unmarshal our byteArray which contains our
		// jsonFile's content into the config map which we defined above
		err = json.Unmarshal(byteValue, &Configurations)
		if err == nil {
			if name == "" {
				name = ProfileName
			}

			// Mark all the profiles as dirty so they will be written back out
			// in the new format.
			for profileName := range Configurations {
				p := Configurations[profileName]
				p.Dirty = true
				p.Name = profileName
				Configurations[profileName] = p
				ui.Log(ui.AppLogger, "Loaded configuration \"%s\" with id %s, %d items", profileName, p.ID, len(p.Items))
			}
		}
	}

	// Get a list of all the profiles (which are named with a file extension of
	// .profile) and load them into the configuration map.
	path = filepath.Join(home, ProfileDirectory)

	if files, err := filepath.Glob(filepath.Join(path, "*.profile")); err == nil {
		for _, file := range files {
			configFile, err := os.Open(file)
			if err == nil {
				ui.Log(ui.AppLogger, "Reading configuration file \"%s\"", file)

				defer configFile.Close()
				byteValue, _ := io.ReadAll(configFile)
				profile := Configuration{}

				err = json.Unmarshal(byteValue, &profile)
				if err == nil {
					shortProfileName := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
					profile.Dirty = false
					profile.Name = shortProfileName
					Configurations[shortProfileName] = &profile

					ui.Log(ui.AppLogger, "Loaded configuration \"%s\" with id %s, %d items", profile.Name, profile.ID, len(profile.Items))
				}
			}
		}
	}

	// Now that we're read all the profile data it, select the requested one as the
	// default configuration.
	cp, found := Configurations[name]
	if !found {
		ui.Log(ui.AppLogger, "No configuration named \"%s\" found, creating", name)

		cp = &Configuration{
			Description: DefaultConfiguration,
			Version:     ConfigurationVersion,
			Items:       map[string]string{},
			Name:        name,
			Dirty:       true,
		}
		Configurations[name] = cp
	} else {
		ui.Log(ui.AppLogger, "Using configuration \"%s\" with id %s", name, cp.ID)
	}

	ProfileName = cp.Name
	CurrentConfiguration = cp

	if err != nil {
		err = errors.New(err)
	}

	return err
}

// Save the current configuration to persistent disk storage.
func Save() error {
	var err error

	for name, profile := range Configurations {
		// So we even need to do anything?
		if !profile.Dirty {
			continue
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

		path = filepath.Join(path, profile.Name+".profile")

		if profile.ID == "" {
			profile.ID = uuid.New().String()
			Configurations[name] = profile
		}

		profile.Dirty = false
		byteBuffer, _ := json.MarshalIndent(&profile, "", "  ")

		err = os.WriteFile(path, byteBuffer, securePermission)
		if err != nil {
			err = errors.New(err)

			ui.Log(ui.AppLogger, "Error storing configuration \"%s\", %v", name, err)

			break
		} else {
			Configurations[name] = profile

			ui.Log(ui.AppLogger, "Stored configuration \"%s\" with id %s to %s", name, profile.ID, path)
		}
	}

	// If everything went okay, delete the old configuration file type if it's still
	// found.
	if errors.Nil(err) {
		if home, err := os.UserHomeDir(); err == nil {
			path := filepath.Join(home, ProfileDirectory, ProfileFile)
			if _, err := os.Stat(path); err == nil {
				_ = os.Remove(path)
			}
		}
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
			Items:       map[string]string{},
			Name:        name,
			Dirty:       true,
		}
		Configurations[name] = c
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
		c.Dirty = true

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
			Name:        "default",
			Items:       map[string]string{}}
	}

	return CurrentConfiguration
}
