package server

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/data"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/validate"
)

var validationDefinitions = map[string]any{
	"@user":                  defs.User{},
	"@credentials":           defs.Credentials{},
	"@dsn":                   defs.DSN{},
	"@dsn.permission":        defs.DSNPermissionItem{},
	"admin.users:post":       "@user",
	"admin.users.name:patch": "@user",
	"dsns:post":              "@dsn",
	"dsns.@permissions:post": "@dsn.permission",
}

func InitializeValidations() {
	var err error

	// Start by creating definitions based on the structure definitions that support the "valid" tag.
	for name, definition := range validationDefinitions {
		if strings.HasPrefix(name, "@") {
			err = validate.Reflect(name, definition)
		} else {
			err = validate.DefineAlias(name, data.String(definition))
		}

		if err != nil {
			ui.Log(ui.ValidationsLogger, "validation.error", ui.A{
				"error": err.Error(),
			})
		} else {
			ui.Log(ui.ValidationsLogger, "validation.defined", ui.A{
				"name": name,
			})
		}
	}

	// Then augment this list by loading validation definitions from the lib/validations path
	if err := loadAllValidations(); err != nil {
		ui.Log(ui.ValidationsLogger, "validation.error", ui.A{
			"error": err.Error(),
		})

		// If thee load failed, ensure the minimum number of validation definitions required for the
		// server to function are defined.
		validate.Define("@credentials", validate.Object{
			Fields: []validate.Item{
				{Name: "username", Type: validate.StringType, Required: true},
				{Name: "password", Type: validate.StringType, Required: true},
				{Name: "expiration", Type: validate.DurationType},
			},
		})

		validate.Define("@permissions", validate.Array{
			Type: validate.Item{
				Type: validate.StringType,
			},
		})

		validate.Define("@user", validate.Object{
			Fields: []validate.Item{
				{
					Name:     "name",
					Required: true,
					Type:     validate.StringType,
				},
				{
					Name: "id",
					Type: validate.UUIDType,
				},
				{
					Name: "password",
					Type: validate.StringType,
				},
				{
					Name: "permissions",
					Type: "@permissions",
				},
			},
		})
	}
}

func loadAllValidations() error {
	path := settings.Get(defs.LibPathName)
	if path == "" {
		path = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
	}

	fn := filepath.Join(path, "validations")

	// Recursively scan the directory for files to pass to validate.LoadDictionary
	err := filepath.WalkDir(fn, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			// The path "env.json" is reserved for use by the main program
			// and is not reloaded here.
			if strings.HasSuffix(path, "env.json") {
				return nil
			}

			err = validate.LoadDictionary(path)
			if err != nil {
				ui.Log(ui.InternalLogger, "validation.load.error", ui.A{
					"path":  path,
					"error": err.Error()})
			} else {
				ui.Log(ui.ValidationsLogger, "validation.loaded", ui.A{
					"path": path})
			}
		}

		return nil
	})

	return err
}
