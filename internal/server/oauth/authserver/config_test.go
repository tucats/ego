package authserver

import (
	"testing"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear all OAuth settings so we get pure defaults.
	settings.Set(defs.OAuthASEnabledSetting, "")
	settings.Set(defs.OAuthASKeyFileSetting, "")
	settings.Set(defs.OAuthASClientFileSetting, "")
	settings.Set(defs.OAuthASIssuerSetting, "")
	settings.Set(defs.OAuthASTokenExpirationSetting, "")
	settings.Set(defs.OAuthASRefreshExpirationSetting, "")
	settings.Set(defs.OAuthASCodeExpirationSetting, "")
	settings.Set(defs.EgoPathSetting, "/tmp/ego-test")

	cfg := loadConfig()

	if cfg.Enabled {
		t.Error("expected Enabled=false with empty setting")
	}

	if cfg.TokenExpiration != defaultTokenExpiration {
		t.Errorf("expected token expiration %v, got %v", defaultTokenExpiration, cfg.TokenExpiration)
	}

	if cfg.RefreshExpiration != defaultRefreshExpiration {
		t.Errorf("expected refresh expiration %v, got %v", defaultRefreshExpiration, cfg.RefreshExpiration)
	}

	if cfg.CodeExpiration != defaultCodeExpiration {
		t.Errorf("expected code expiration %v, got %v", defaultCodeExpiration, cfg.CodeExpiration)
	}
}

func TestLoadConfig_ExplicitValues(t *testing.T) {
	settings.Set(defs.OAuthASEnabledSetting, "true")
	settings.Set(defs.OAuthASIssuerSetting, "https://ego.example.com")
	settings.Set(defs.OAuthASKeyFileSetting, "/tmp/test-key.pem")
	settings.Set(defs.OAuthASClientFileSetting, "/tmp/test-clients.json")
	settings.Set(defs.OAuthASTokenExpirationSetting, "30m")
	settings.Set(defs.OAuthASRefreshExpirationSetting, "48h")
	settings.Set(defs.OAuthASCodeExpirationSetting, "2m")

	cfg := loadConfig()

	if !cfg.Enabled {
		t.Error("expected Enabled=true")
	}

	if cfg.Issuer != "https://ego.example.com" {
		t.Errorf("unexpected issuer: %s", cfg.Issuer)
	}

	if cfg.KeyFile != "/tmp/test-key.pem" {
		t.Errorf("unexpected key file: %s", cfg.KeyFile)
	}

	if cfg.TokenExpiration != 30*time.Minute {
		t.Errorf("expected 30m token expiration, got %v", cfg.TokenExpiration)
	}

	if cfg.RefreshExpiration != 48*time.Hour {
		t.Errorf("expected 48h refresh expiration, got %v", cfg.RefreshExpiration)
	}

	if cfg.CodeExpiration != 2*time.Minute {
		t.Errorf("expected 2m code expiration, got %v", cfg.CodeExpiration)
	}
}

func TestLoadConfig_InvalidDuration_FallsBackToDefault(t *testing.T) {
	settings.Set(defs.OAuthASTokenExpirationSetting, "not-a-duration")
	settings.Set(defs.OAuthASRefreshExpirationSetting, "")
	settings.Set(defs.OAuthASCodeExpirationSetting, "")

	cfg := loadConfig()

	if cfg.TokenExpiration != defaultTokenExpiration {
		t.Errorf("expected default token expiration after invalid value, got %v", cfg.TokenExpiration)
	}
}
