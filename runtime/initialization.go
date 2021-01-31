package runtime

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/defs"
)

func InitProfileDefaults() error {

	egopath, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	// The default values we check for.
	settings := map[string]string{
		defs.EgoPathSetting:           egopath,
		defs.AutoImportSetting:        "true",
		defs.CaseNormalizedSetting:    "false",
		defs.StaticTypesSetting:       "dynamic",
		defs.OutputFormatSetting:      "text",
		defs.ExtensionsEnabledSetting: "false",
		defs.UseReadline:              "true",
		defs.TokenExpirationSetting:   "24h",
		defs.TokenKeySetting:          strings.ReplaceAll(uuid.New().String()+uuid.New().String(), "-", ""),
		defs.ExitOnBlankSetting:       "false",
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
