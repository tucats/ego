package runtime

import (
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/defs"
	"github.com/tucats/gopackages/app-cli/persistence"
)

func InitProfileDefaults() error {

	// The default values we check for.
	settings := map[string]string{
		defs.AutoImportSetting:      "true",
		defs.CaseNormalizedSetting:  "false",
		defs.OutputFormatSetting:    "text",
		defs.PrintEnabledSetting:    "false",
		defs.UseReadline:            "true",
		defs.TokenExpirationSetting: "24h",
		defs.TokenKeySetting:        strings.ReplaceAll(uuid.New().String()+uuid.New().String(), "-", ""),
		defs.ExitOnBlankSetting:     "false",
	}

	// See if there is a value for each on of these. If no
	// value, set the default value.
	dirty := false
	var err error

	for k, v := range settings {
		if !persistence.Exists(k) {
			persistence.Set(k, v)
			dirty = true
		}
	}
	if dirty {
		err = persistence.Save()
	}
	return err
}
