package profile

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
)

const (
	ServerDefaults  = 1
	RuntimeDefaults = 2
	AllDefaults     = ServerDefaults + RuntimeDefaults
)

type configInitializer struct {
	class int
	value string
}

func InitProfileDefaults(class int) error {
	var (
		err         error
		serverToken string
	)

	egopath, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	// The initialzier for the pid directory is platform-specific.
	homedir, _ := os.UserHomeDir()
	piddir := path.Join(homedir, settings.ProfileDirectory)

	// If it doesn't already exist, generate a random key string for the server token.
	// If for some reason this fails, generate a less secure key from UUID values.
	if class == AllDefaults || class == ServerDefaults {
		if existingToken := settings.Get(defs.ServerTokenKeySetting); existingToken == "" {
			serverToken = "U"
			token := make([]byte, 256)

			if _, err := rand.Read(token); err != nil {
				for len(serverToken) < 512 {
					serverToken += strings.ReplaceAll(uuid.New().String(), "-", "")
				}
			} else {
				// Convert the token byte array to a hex string.
				serverToken = strings.ToLower(hex.EncodeToString(token))
			}

			shortToken := serverToken
			if len(shortToken) > 9 {
				shortToken = shortToken[:4] + "..." + shortToken[len(shortToken)-4:]
			}

			ui.Log(ui.AppLogger, "app.new.server.token", ui.A{
				"token":   shortToken,
				"profile": settings.ActiveProfileName()})
		}
	}

	// The default values we check for.
	var initialSettings = map[string]configInitializer{
		defs.EgoPathSetting:                {RuntimeDefaults, egopath},
		defs.AutoImportSetting:             {RuntimeDefaults, defs.True},
		defs.CaseNormalizedSetting:         {RuntimeDefaults, defs.False},
		defs.StaticTypesSetting:            {RuntimeDefaults, defs.Dynamic},
		defs.UnusedVarsSetting:             {RuntimeDefaults, defs.True},
		defs.UnknownVarSetting:             {RuntimeDefaults, defs.False},
		defs.UnusedVarLoggingSetting:       {RuntimeDefaults, defs.False},
		defs.ServerReportFQDNSetting:       {ServerDefaults, defs.False},
		defs.OutputFormatSetting:           {RuntimeDefaults, ui.TextFormat},
		defs.ExtensionsEnabledSetting:      {RuntimeDefaults, defs.False},
		defs.UseReadline:                   {RuntimeDefaults, defs.True},
		defs.ServerTokenExpirationSetting:  {ServerDefaults, "24h"},
		defs.ServerTokenKeySetting:         {ServerDefaults, serverToken},
		defs.ThrowUncheckedErrorsSetting:   {RuntimeDefaults, defs.True},
		defs.FullStackTraceSetting:         {RuntimeDefaults, defs.False},
		defs.LogTimestampFormat:            {ServerDefaults, "2006-01-02 15:04:05"},
		defs.PidDirectorySetting:           {RuntimeDefaults, piddir},
		defs.InsecureServerSetting:         {RuntimeDefaults, defs.False},
		defs.RestClientErrorSetting:        {RuntimeDefaults, defs.True},
		defs.LogRetainCountSetting:         {ServerDefaults, "3"},
		defs.TablesServerEmptyFilterError:  {ServerDefaults, defs.True},
		defs.TablesServerEmptyRowsetError:  {ServerDefaults, defs.True},
		defs.TableServerPartialInsertError: {ServerDefaults, defs.True},
		defs.SymbolTableAllocationSetting:  {RuntimeDefaults, "32"},
		defs.ExecPermittedSetting:          {RuntimeDefaults, defs.False},
		defs.PrecisionErrorSetting:         {RuntimeDefaults, defs.True},
		defs.RestClientTimeoutSetting:      {RuntimeDefaults, "10s"},
		defs.TableAutoparseDSN:             {ServerDefaults, defs.True},
		defs.RuntimeDeepScopeSetting:       {RuntimeDefaults, defs.True},
	}

	// See if there is a value for each on of these. If no
	// value, set the default value.
	dirty := false

	// For all the default values we know about, set them if they don't exist.
	for settingName, defaultValue := range initialSettings {
		// If a specific class was specified and this item isn't in the class, skip it.
		if class != AllDefaults && defaultValue.class != class {
			continue
		}

		// If it's not already set, then set it to the default value.
		if !settings.Exists(settingName) {
			settings.Set(settingName, defaultValue.value)

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
