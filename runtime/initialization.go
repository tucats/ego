package runtime

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/persistence"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

func InitProfileDefaults() *errors.EgoError {
	var err *errors.EgoError

	egopath, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	// The default values we check for.
	settings := map[string]string{
		defs.EgoPathSetting:              egopath,
		defs.AutoImportSetting:           "true",
		defs.CaseNormalizedSetting:       "false",
		defs.StaticTypesSetting:          "dynamic",
		defs.OutputFormatSetting:         "text",
		defs.ExtensionsEnabledSetting:    "false",
		defs.UseReadline:                 "true",
		defs.TokenExpirationSetting:      "24h",
		defs.TokenKeySetting:             strings.ReplaceAll(uuid.New().String()+uuid.New().String(), "-", ""),
		defs.ExitOnBlankSetting:          "false",
		defs.ThrowUncheckedErrorsSetting: "true",
		defs.FullStackTraceSetting:       "false",
		defs.LogTimestampFormat:          "2006-01-02 15:04:05",
		defs.PidDirectorySetting:         "/var/run/ego/",
	}

	// See if there is a value for each on of these. If no
	// value, set the default value.
	dirty := false

	for k, v := range settings {
		if !persistence.Exists(k) {
			persistence.Set(k, v)

			dirty = true
		}
	}

	if dirty {
		err = persistence.Save()
	}

	// Patch up some things now that we have a stable profile
	if fmtstring := persistence.Get(defs.LogTimestampFormat); fmtstring != "" {
		ui.LogTimeStampFormat = fmtstring
	}

	return err
}
