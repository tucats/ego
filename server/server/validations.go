package server

import (
	"os"
	"path/filepath"

	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/validate"
)

func InitializeValidations() {
	path := settings.Get(defs.LibPathName)
	if path == "" {
		path = filepath.Join(settings.Get(defs.EgoPathSetting), defs.LibPathName)
	}

	fn := filepath.Join(path, "validations.json")
	if err := validate.LoadDictionary(fn); err != nil {
		if !os.IsNotExist(err) {
			ui.Log(ui.InternalLogger, "validation.load.error", ui.A{
				"path":  fn,
				"error": err})
		}

		validate.Define("@credentials", validate.Object{
			Fields: []validate.Item{
				{Name: "username", Type: validate.StringType, Required: true},
				{Name: "password", Type: validate.StringType, Required: true},
				{Name: "expiration", Type: validate.StringType},
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
	} else {
		ui.Log(ui.ServerLogger, "validation.loaded", ui.A{
			"path": fn})
	}
}
