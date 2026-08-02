package util

import (
	"net/url"
	"testing"

	"github.com/tucats/ego/internal/errors"
)

// The "list" parameter type accepts a parameter that is present but empty
// (?class=), reading it the same way the "bool" and "duration" types already
// read an empty value: as "the caller is not filtering on this". That lets a UI
// build a query string from a set of fields without having to drop a parameter
// from the URL entirely when its filter is cleared.
//
// Every handler that reads a list parameter has to agree with that reading --
// an empty list must mean "no filter", never "match nothing" or "exclude
// everything". The handler-side halves of this are tested next to those
// handlers; what is pinned here is the validation contract they depend on.
func TestValidateListParameterAcceptsEmptyValue(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantError bool
	}{
		{
			name: "a named value is accepted",
			url:  "http://localhost:8080/services/admin/log?class=REST",
		},
		{
			name: "several comma-separated values are accepted",
			url:  "http://localhost:8080/services/admin/log?class=REST,AUTH",
		},
		{
			name: "the parameter repeated is accepted",
			url:  "http://localhost:8080/services/admin/log?class=REST&class=AUTH",
		},
		{
			name: "present but empty is accepted and means no filter",
			url:  "http://localhost:8080/services/admin/log?class=",
		},
		{
			name: "absent entirely is accepted",
			url:  "http://localhost:8080/services/admin/log?tail=50",
		},
		{
			name: "repeated with one empty value is accepted",
			url:  "http://localhost:8080/services/admin/log?class=REST&class=",
		},
	}

	validation := map[string]string{"class": ListParameterType, "tail": IntParameterType}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("could not parse test URL: %v", err)
			}

			got := ValidateParameters(parsed, validation)

			if tt.wantError && errors.Nil(got) {
				t.Errorf("expected an error for %s, got none", tt.url)
			}

			if !tt.wantError && !errors.Nil(got) {
				t.Errorf("expected no error for %s, got %v", tt.url, got)
			}
		})
	}
}

// An empty INT parameter is still rejected. Unlike an empty list, an empty
// integer has no obvious reading -- it is not "zero" and not "no filter", it is
// simply malformed -- so the caller is better told about it.
func TestValidateIntParameterStillRejectsEmptyValue(t *testing.T) {
	parsed, err := url.Parse("http://localhost:8080/services/admin/log?tail=")
	if err != nil {
		t.Fatalf("could not parse test URL: %v", err)
	}

	if got := ValidateParameters(parsed, map[string]string{"tail": IntParameterType}); errors.Nil(got) {
		t.Error("an empty int parameter should be rejected, but was accepted")
	}
}
