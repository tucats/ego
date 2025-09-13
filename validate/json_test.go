package validate

import "testing"

func TestValidateSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    any    // Object, Array, or Item
		json    string // JSON data to be validated
		wantErr bool
	}{
		{
			name: "invalid usernane/password object, misspelled field",
			spec: Object{
				Fields: []Item{
					{
						Type:     StringType,
						Name:     "username",
						Required: true,
					},
					{
						Type:     StringType,
						Name:     "password",
						Required: true,
					},
				},
			},
			json:    `{"username": "bob", "pass": "secret"}`,
			wantErr: true,
		},
		{
			name: "invalid usernane/password object, missing field",
			spec: Object{
				Fields: []Item{
					{
						Type:     StringType,
						Name:     "username",
						Required: true,
					},
					{
						Type:     StringType,
						Name:     "password",
						Required: true,
					},
				},
			},
			json:    `{"username": "bob"}`,
			wantErr: true,
		},
		{
			name: "valid usernane/password object",
			spec: Object{
				Fields: []Item{
					{
						Type:     StringType,
						Name:     "username",
						Required: true,
					},
					{
						Type:     StringType,
						Name:     "password",
						Required: true,
					},
				},
			},
			json:    `{"username": "bob", "password": "secret"}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateWithSpec([]byte(tt.json), tt.spec); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
