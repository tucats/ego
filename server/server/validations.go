package server

import "github.com/tucats/ego/validate"

func InitializeValidations() {
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
}
