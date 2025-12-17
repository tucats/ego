package settings

import "github.com/tucats/ego/i18n"

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

// DefaultConfiguration is a localized string that contains the
// local text for "Default configuration".
var DefaultConfiguration = i18n.L("Default.configuration")

// Configuration version number. This is used to ensure that the
// configuration file is compatible with the current version of the
// application. The default is zero.
const ConfigurationVersion = 0
