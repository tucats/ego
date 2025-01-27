// Package settings manages the persistent user profile used by the command
// application infrastructure. This includes automatically reading any
// profile in as part of startup, and of updating the profile as needed.
package settings

import (
	"encoding/json"
	goerr "errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
)

const encryptionPrefixTag = "encrypted: "

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

	// Random value used for encryption for this configuration.
	Salt string `json:"salt,omitempty"`

	// The Items map contains the individual configuration values. Each
	// has a key which is the name of the option, and a string value for
	// that configuration item. Configuration items that are not strings
	// must be serialized as a string.
	Items map[string]string `json:"items"`
}

// CurrentConfiguration describes the current configuration that is active.
var CurrentConfiguration *Configuration

// Map of config tokens that are stored in separate files from the main
// configuration. The "$" is a placeholder for the profile name in the
// file name.
var fileMapping = map[string]string{
	"ego.logon.token":      "$.token",
	"ego.server.token.key": "$.key",
}

// Load reads in the named profile, if it exists.
func Load(application string, name string) error {
	var c = Configuration{
		Description: DefaultConfiguration,
		Version:     ConfigurationVersion,
		Name:        name,
		Dirty:       true,
		Items:       map[string]string{},
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return errors.New(err)
	}

	ui.Log(ui.AppLogger, "config.active", ui.A{
		"name": name})

	// Do we already have that configuration loaded? If so make it current
	// and we're done.
	if c, ok := Configurations[name]; ok {
		path := filepath.Join(home, ProfileDirectory, name+".profile")
		CurrentConfiguration = c

		ui.Log(ui.AppLogger, "config.base.loaded", ui.A{
			"path": path})

		// For any keys that are stored as separate file values, get them now.
		readOutboardConfigFiles(home, name, c)

		ui.Log(ui.AppLogger, "config.is.active", ui.A{
			"Name": CurrentConfiguration.Name,
			"id":   CurrentConfiguration.ID})

		return nil
	}

	// Nope, need to create the configuration structure and populate from
	// disk before we can make it current.
	CurrentConfiguration = &c
	Configurations = map[string]*Configuration{"default": CurrentConfiguration}
	ProfileFile = application + ".json"

	// First, make sure the profile directory exists.
	path := filepath.Join(home, ProfileDirectory)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		_ = os.MkdirAll(path, securePermission)
		ui.Log(ui.AppLogger, "config.create.dir", ui.A{
			"path": path})
	}

	// Read legacy configuration file if it exists.
	err, name = readLegacyConfigFormat(path, home, name)

	// Get a list of all the profiles (which are named with a file extension of
	// .profile) and load them into the configuration map.
	path = filepath.Join(home, ProfileDirectory)

	if files, err := filepath.Glob(filepath.Join(path, "*.profile")); err == nil {
		for _, file := range files {
			configFile, err := os.Open(file)
			if err == nil {
				ui.Log(ui.AppLogger, "config.read.profile", ui.A{
					"path": file})

				defer configFile.Close()
				byteValue, _ := io.ReadAll(configFile)
				profile := Configuration{}

				err = json.Unmarshal(byteValue, &profile)
				if err == nil {
					shortProfileName := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
					profile.Dirty = false
					profile.Name = shortProfileName
					Configurations[shortProfileName] = &profile

					ui.Log(ui.AppLogger, "config.loaded.config", ui.A{
						"name":  profile.Name,
						"id":    profile.ID,
						"count": len(profile.Items)})
				}
			}
		}
	}

	// Now that we're read all the profile data it, select the requested one as the
	// default configuration.
	cp, found := Configurations[name]
	if !found {
		ui.Log(ui.AppLogger, "config.not.found", ui.A{
			"name": name})

		cp = &Configuration{
			Description: DefaultConfiguration,
			Version:     ConfigurationVersion,
			Items:       map[string]string{},
			Name:        name,
			Dirty:       true,
		}
		Configurations[name] = cp
	} else {
		ui.Log(ui.AppLogger, "config.using", ui.A{
			"name": name,
			"id":   cp.ID})
	}

	ProfileName = cp.Name
	CurrentConfiguration = cp

	// If the salt value is empty or not set, generate it now.
	if cp.Salt == "" {
		cp.Salt = strings.ReplaceAll(uuid.NewString()+uuid.NewString(), "-", "")

		ui.Log(ui.AppLogger, "config.salt")

		cp.Dirty = true
	}

	// Last step; for any keys that are stored as separate file values, get them now.
	readOutboardConfigFiles(home, name, cp)

	// Patch up anything that should be changed by the newly loaded configuration.
	if err == nil {
		// ego.console.log
		if value, found := cp.Items[defs.LogFormatSetting]; found {
			if value == "json" || value == "text" {
				ui.LogFormat = value
			}
		}

		// ego.console.output
		if value, found := cp.Items[defs.OutputFormatSetting]; found {
			if value == "json" || value == "text" || value == "indented" {
				ui.OutputFormat = value
			}
		}
	} else {
		err = errors.New(err)
	}

	return err
}

func readOutboardConfigFiles(home string, name string, cp *Configuration) {
	for token, file := range fileMapping {
		fileName := filepath.Join(home, ProfileDirectory, strings.Replace(file, "$", name, 1))

		bytes, err := os.ReadFile(fileName)
		if err == nil {
			var value string

			ui.Log(ui.AppLogger, "config.external", ui.A{
				"name": token,
				"path": fileName})

			err := json.Unmarshal(bytes, &value)
			if err == nil && len(value) > 0 {
				// Decrypt the value using the salt as the password if it is marked as an encrypted value.
				if strings.HasPrefix(value, encryptionPrefixTag) {
					value, err = Decrypt(strings.TrimPrefix(value, encryptionPrefixTag), cp.Name+cp.Salt+cp.ID)
					if err != nil {
						ui.Log(ui.AppLogger, "config.decrypt.error", ui.A{
							"name":  token,
							"error": err})

						continue
					} else {
						ui.Log(ui.AppLogger, "config.decrypted", ui.A{
							"name": token})
					}
				}

				// Save the decrypted value to the configuration.
				cp.Items[token] = value
			}
		}
	}
}

// If it's found, read the old grouped configurations from the profile file.
func readLegacyConfigFormat(path string, home string, name string) (error, string) {
	path = filepath.Join(home, ProfileDirectory, ProfileFile)

	// Try to open the file.
	configFile, err := os.Open(path)
	if err == nil {
		// read the json config as a byte array.
		defer configFile.Close()
		ui.Log(ui.AppLogger, "config.legacy.read", ui.A{
			"path": path})

		byteValue, _ := io.ReadAll(configFile)

		err = json.Unmarshal(byteValue, &Configurations)
		if err == nil {
			if name == "" {
				name = ProfileName
			}

			for profileName := range Configurations {
				p := Configurations[profileName]
				// Mark all the profiles as dirty so they will be written back out
				// in the new format.
				p.Dirty = true
				p.Name = profileName
				Configurations[profileName] = p
				ui.Log(ui.AppLogger, "config.legacy.load", ui.A{
					"name":  profileName,
					"id":    p.ID,
					"count": len(p.Items)})
			}
		}
	} else {
		// Couldn't read it, but we don't worry about that.
		err = nil
	}

	return err, name
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

		var savedItems = map[string]string{}

		// First, process any profile items intended to be stored as separate file values.
		saveOutboardConfigItems(profile, home, name, err, savedItems)

		// Now write the combined configuration to the profile file, having omitted
		// the key values already extracted to separate files.
		profile.Dirty = false
		byteBuffer, _ := json.MarshalIndent(profile, ui.JSONIndentPrefix, ui.JSONIndentSpacer)

		// Restore the saved items that had been written to external files back into
		// the active profile.
		for token, value := range savedItems {
			profile.Items[token] = value
		}

		// Finally, write the updated configuration to the profile file.
		err = os.WriteFile(path, byteBuffer, securePermission)
		if err != nil {
			err = errors.New(err)

			ui.Log(ui.AppLogger, "config.save.error", ui.A{
				"name":  name,
				"error": err})

			break
		} else {
			Configurations[name] = profile

			ui.Log(ui.AppLogger, "config.save", ui.A{
				"name": name,
				"id":   profile.ID,
				"path": path})
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

// Write any keys that are intended to be stored outside the configuration into separate files.
func saveOutboardConfigItems(profile *Configuration, home string, name string, err error, savedItems map[string]string) {
	for token, file := range fileMapping {
		ui.Log(ui.AppLogger, "config.external.check", ui.A{
			"name": token})

		// We only do this for key values that exist and are non-empty.
		if value, ok := profile.Items[token]; ok && len(value) > 0 {
			fileName := filepath.Join(home, ProfileDirectory, strings.Replace(file, "$", name, 1))

			// Encrypt the value using the salt as the password
			value, err = Encrypt(value, profile.Name+profile.Salt+profile.ID)
			if err != nil {
				ui.Log(ui.AppLogger, "config.external.encrypt.error", ui.A{
					"name":  token,
					"error": err})

				continue
			} else {
				ui.Log(ui.AppLogger, "config.external.encxrypt", ui.A{
					"name": token})

				value = encryptionPrefixTag + value
			}

			bytes, err := json.MarshalIndent(value, ui.JSONIndentPrefix, ui.JSONIndentSpacer)
			if err == nil {
				// First see if the file already exists and contains the same value. If
				// so we do not write it again.
				oldBytes, err := os.ReadFile(fileName)
				if err == nil && reflect.DeepEqual(oldBytes, bytes) {
					continue
				}

				err = os.WriteFile(fileName, bytes, securePermission)
				if err != nil {
					err = errors.New(err)

					ui.Log(ui.AppLogger, "config.external.write.error", ui.A{
						"name":  token,
						"path":  fileName,
						"error": err})

					break
				} else {
					savedItems[token] = profile.Items[token]

					delete(profile.Items, token)
					ui.Log(ui.AppLogger, "config.extenral.write", ui.A{
						"name": token,
						"path": fileName})
				}
			}
		} else {
			// This config item doesn't exist in the configuration, so make sure there isn't
			// a correponsing outboard file that should be deleted.
			fileName := filepath.Join(home, ProfileDirectory, strings.Replace(file, "$", name, 1))

			err := os.Remove(fileName)
			if err == nil {
				ui.Log(ui.AppLogger, "config.external.deleted", ui.A{
					"path": fileName})
			} else if !goerr.Is(err, fs.ErrNotExist) {
				ui.Log(ui.AppLogger, "config.external.delete.error", ui.A{
					"path":  fileName,
					"error": err})
			} else {
				ui.Log(ui.AppLogger, "config.external.not.found", ui.A{
					"path": fileName})
			}
		}
	}
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
			ui.Log(ui.AppLogger, "config.delete.active", ui.A{
				"name": key})

			return errors.ErrCannotDeleteActiveProfile.Context(key)
		}

		delete(Configurations, key)

		home, err := os.UserHomeDir()
		if err != nil {
			return errors.New(err)
		}

		// Attempt to delete the physical file. If it can't be deleted, log
		// the error but continue with the deletion of the profile from the
		// in-memory map. We also use this as a chance to see if any other
		// profiles need to be refreshed on disk.
		path := filepath.Join(home, ProfileDirectory, key+".profile")
		if err = os.Remove(path); err != nil {
			ui.Log(ui.AppLogger, "config.delete.error", ui.A{
				"path":  path,
				"error": err})
		} else {
			err = Save()
		}

		// See if there are any externalized values in files that also need to be
		// deleted.
		for _, file := range fileMapping {
			fileName := filepath.Join(home, ProfileDirectory, strings.Replace(file, "$", key, 1))
			if _, err := os.Stat(fileName); err == nil {
				err = os.Remove(fileName)
				if err == nil {
					ui.Log(ui.AppLogger, "config.deleted", ui.A{
						"name": key,
						"path": fileName})
				} else {
					ui.Log(ui.AppLogger, "config.external.delete.error", ui.A{
						"path":  fileName,
						"error": err})
				}
			}
		}

		// If the deletion was successful, log the deletion.
		if err == nil {
			ui.Log(ui.AppLogger, "config.deleted", ui.A{
				"name": key,
				"path": path})
		}

		return err
	}

	ui.Log(ui.AppLogger, "config.delete.not.found", ui.A{
		"name": key})

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
