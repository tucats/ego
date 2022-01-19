package runtime

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

func InitProfileDefaults() *errors.EgoError {
	var err *errors.EgoError

	egopath, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	// The initialzier for the pid directory is platform-specific.
	homedir, _ := os.UserHomeDir()
	piddir := path.Join(homedir, ".org.fernwood")

	// The default values we check for.
	initialSettings := map[string]string{
		defs.EgoPathSetting:               egopath,
		defs.AutoImportSetting:            defs.True,
		defs.CaseNormalizedSetting:        defs.False,
		defs.StaticTypesSetting:           "dynamic",
		defs.OutputFormatSetting:          ui.TextFormat,
		defs.ExtensionsEnabledSetting:     defs.False,
		defs.UseReadline:                  defs.True,
		defs.ServerTokenExpirationSetting: "24h",
		defs.ServerTokenKeySetting:        strings.ReplaceAll(uuid.New().String()+uuid.New().String(), "-", ""),
		defs.ExitOnBlankSetting:           defs.False,
		defs.ThrowUncheckedErrorsSetting:  defs.True,
		defs.FullStackTraceSetting:        defs.False,
		defs.LogTimestampFormat:           "2006-01-02 15:04:05",
		defs.PidDirectorySetting:          piddir,
		defs.InsecureServerSetting:        defs.False,
	}

	// See if there is a value for each on of these. If no
	// value, set the default value.
	dirty := false

	for k, v := range initialSettings {
		if !settings.Exists(k) {
			settings.Set(k, v)

			dirty = true
		}
	}

	if dirty {
		err = settings.Save()
	}

	// Patch up some things now that we have a stable profile
	if fmtstring := settings.Get(defs.LogTimestampFormat); fmtstring != "" {
		ui.LogTimeStampFormat = fmtstring
	}

	return err
}
