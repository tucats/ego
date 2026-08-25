package router

// features.go implements GET /services/admin/features, a small, credential-free
// probe endpoint the dashboard (or any other client) can call to discover which
// optional server capabilities are turned on, without needing the root-only
// GET/POST /admin/config to see the underlying settings. Modeled on
// WebAuthnConfigHandler in webauthn.go, which does the same thing for passkeys.
//
// Add a new boolean capability to the Features map, or a new named value as
// its own response field alongside Timezone, rather than adding a new endpoint.

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/util"
)

// FeaturesHandler reports which optional server capabilities are currently
// configured. Each boolean entry in Features answers "is this usable right
// now", not "does this build support it" -- for example "ai" is true only
// when ego.server.ai.model has been set to a non-empty model name.
func FeaturesHandler(session *Session, w http.ResponseWriter, r *http.Request) int {
	w.Header().Set(defs.ContentTypeHeader, defs.JSONMediaType)

	resp := struct {
		Server   defs.ServerInfo `json:"server"`
		Msg      string          `json:"msg"`
		Status   int             `json:"status"`
		Timezone string          `json:"timezone"`
		Features map[string]bool `json:"features"`
	}{
		Server:   util.MakeServerInfo(session.ID),
		Timezone: resolveTimezoneName(),
		Features: map[string]bool{
			"ai":         settings.Get(defs.ServerAIModelSetting) != "",
			"biometrics": settings.GetBool(defs.WebAuthnAllowPasskeysSetting),
		},
		Status: http.StatusOK,
		Msg:    "",
	}

	b := util.WriteJSON(w, session.Response(), http.StatusOK, resp)
	ui.Log(ui.RestLogger, "rest.response.payload", ui.A{
		"session": session.ID,
		"body":    string(b),
	})

	return http.StatusOK
}

// resolveTimezoneName reports the timezone a caller should assume this
// server means by a bare or local timestamp: the ego.runtime.timezone
// setting's value when it names a zone explicitly, or -- when the setting is
// unset or the word "local" -- the best available name for the host's own
// zone, so the caller has the best chance of correctly interpreting a time
// value rather than just being told "local" (which is literally what
// time.Local.String() returns, and names nothing).
//
// The host's zone is synthesized in the same order Go itself resolves
// time.Local (see util.DefaultLocation's comment): the TZ environment
// variable, then the /etc/localtime symlink target (both give a real IANA
// name, which is what a caller needs to interpret timestamps correctly
// across a DST transition, not just at the current moment). When neither
// source is available -- a minimal container, or a platform such as Windows
// with no /etc/localtime -- there is no way to recover a real zone name, so
// the current numeric UTC offset is reported instead as the best remaining
// answer.
func resolveTimezoneName() string {
	configured := strings.TrimSpace(settings.Get(defs.RuntimeTimeZoneSetting))
	if configured != "" && !strings.EqualFold(configured, defs.LocalTimeZone) {
		return configured
	}

	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}

	if target, err := os.Readlink("/etc/localtime"); err == nil {
		const zoneinfoDir = "zoneinfo/"
		if idx := strings.Index(target, zoneinfoDir); idx >= 0 {
			return target[idx+len(zoneinfoDir):]
		}
	}

	return time.Now().Format("-07:00")
}
