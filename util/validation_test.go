package util

import (
	"net/url"
	"testing"

	"github.com/tucats/ego/errors"
)

func TestValidateParameters(t *testing.T) {
	tests := []struct {
		name          string
		validation    map[string]string
		expectedError bool
	}{
		{
			name: "http://localhost:8080/tables/list?start=1&count=5",
			validation: map[string]string{
				"start": "int",
				"count": "int",
			},
			expectedError: false,
		},
		{
			name: "http://localhost:8080/tables/list?start=1&count=5&illegal=true",
			validation: map[string]string{
				"start": "int",
				"count": "int",
			},
			expectedError: true,
		},
		{
			name: "http://localhost:8080/tables/list?start=1&full",
			validation: map[string]string{
				"start": "int",
				"full":  "flag",
			},
			expectedError: false,
		},
		{
			name: "http://localhost:8080/tables/list?start=test&full",
			validation: map[string]string{
				"start": "int",
				"full":  "flag",
			},
			expectedError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, _ := url.Parse(tt.name)

			if got := ValidateParameters(url, tt.validation); errors.Nil(got) == tt.expectedError {
				t.Errorf("ValidateParameters() = %v, want %v", got, tt.expectedError)
			}
		})
	}
}
