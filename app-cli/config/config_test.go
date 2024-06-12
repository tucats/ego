package config

import (
	"testing"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/settings"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
)

func TestSetOutputAction_ValidOutputFormat(t *testing.T) {
	c := &cli.Context{
		Parameters: []string{"json"},
	}

	err := SetOutputAction(c)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	outputFormat := settings.Get(defs.OutputFormatSetting)
	if outputFormat != ui.JSONFormat {
		t.Errorf("Expected output format to be %s, but got: %s", ui.JSONFormat, outputFormat)
	}
}

func TestSetOutputAction_InvalidOutputFormat(t *testing.T) {
	c := &cli.Context{
		Parameters: []string{"invalid"},
	}

	err := SetOutputAction(c)
	if err == nil {
		t.Error("Expected an error, but got none")
	}

	expectedError := errors.ErrInvalidOutputFormat.Context("invalid")
	if err.Error() != expectedError.Error() {
		t.Errorf("Expected error: %v, but got: %v", expectedError, err)
	}
}

func TestSetOutputAction_MissingOutputType(t *testing.T) {
	c := &cli.Context{
		Parameters: []string{},
	}

	err := SetOutputAction(c)
	if err == nil {
		t.Error("Expected an error, but got none")
	}

	expectedError := errors.ErrMissingOutputType
	if err.Error() != expectedError.Error() {
		t.Errorf("Expected error: %v, but got: %v", expectedError, err)
	}
}

func TestSetOutputAction_MultipleParameters(t *testing.T) {
	c := &cli.Context{
		Parameters: []string{"json", "extra"},
	}

	err := SetOutputAction(c)
	if err == nil {
		t.Error("Expected an error, but got none")
	}

	expectedError := errors.ErrMissingOutputType
	if err.Error() != expectedError.Error() {
		t.Errorf("Expected error: %v, but got: %v", expectedError, err)
	}
}

func TestSetOutputAction_EmptyParameters(t *testing.T) {
	c := &cli.Context{
		Parameters: nil,
	}

	err := SetOutputAction(c)
	if err == nil {
		t.Error("Expected an error, but got none")
	}

	expectedError := errors.ErrMissingOutputType
	if err.Error() != expectedError.Error() {
		t.Errorf("Expected error: %v, but got: %v", expectedError, err)
	}
}

func TestSetDescriptionAction(t *testing.T) {
	// Test case 1: Set description for an existing profile
	settings.Configurations = map[string]*settings.Configuration{
		"testProfile": {
			Name:        "testProfile",
			Description: "Original Description",
			Dirty:       false,
		},
	}
	settings.ProfileName = "testProfile"

	c := &cli.Context{
		Parameters: []string{"New Description"},
	}

	err := SetDescriptionAction(c)
	if err != nil {
		t.Errorf("Test case 1 failed: %v", err)
	}

	expectedDescription := "New Description"
	if settings.Configurations["testProfile"].Description != expectedDescription {
		t.Errorf("Test case 1 failed: expected description '%s', got '%s'", expectedDescription, settings.Configurations["testProfile"].Description)
	}

	// Test case 2: Set description for a non-existing profile
	settings.Configurations = map[string]*settings.Configuration{}
	settings.ProfileName = "nonExistentProfile"

	err = SetDescriptionAction(c)
	if err == nil {
		t.Error("Test case 2 failed: expected error, got nil")
	}

	// Test case 3: Set description with no parameters
	c = &cli.Context{
		Parameters: []string{},
	}

	err = SetDescriptionAction(c)
	if err == nil {
		t.Error("Test case 3 failed: expected error, got nil")
	}

	// Test case 4: Set description with multiple parameters
	c = &cli.Context{
		Parameters: []string{"New Description", "Extra Parameter"},
	}

	err = SetDescriptionAction(c)
	if err == nil {
		t.Error("Test case 4 failed: expected error, got nil")
	}

	// Test case 5: Set description with an empty string
	c = &cli.Context{
		Parameters: []string{""},
	}

	err = SetDescriptionAction(c)
	if err == nil {
		t.Error("Test case 5 failed: expected error, got nil")
	}
}
