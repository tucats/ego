package admin

import (
	"bytes"
	"net/http"

	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/router"
	"github.com/tucats/ego/internal/server/assets"
	"github.com/tucats/ego/internal/util"
)

// langPlaceholder is the sentinel string embedded in dashboard.html that marks
// every location where the negotiated language code must be substituted before
// the page is sent to the browser.  It appears in two places in the template:
//
//  1. The <html lang="…"> attribute — tells browsers and screen readers which
//     language the page is written in.
//  2. The <meta name="ego-lang" content="…"> tag — read by dashboard.js at
//     startup so that every subsequent API call includes the matching
//     Accept-Language header (see the apiFetch function in dashboard.js).
const langPlaceholder = "__EGO_LANG__"

// UIHandler launches the dashboard UI. It loads dashboard.html from the local
// asset store, substitutes the language code (or empty string) for every
// occurrence of langPlaceholder, and writes the resulting HTML to the response.
//
// Language handling deliberately avoids assuming any server-side default:
//
//   - If the caller supplies ?lang=fr (or any other supported code), that code
//     is injected into the page and dashboard.js will send it as the
//     Accept-Language header on every subsequent API call.
//
//   - If ?lang= is absent, empty, or names a language Ego has no translations
//     for, the placeholder is replaced with an empty string.  dashboard.js then
//     omits the Accept-Language header entirely, allowing the browser to send
//     its own native Accept-Language value — which reflects the user's actual
//     OS/browser language settings — on every API request.
//
// The second behavior is intentional: we must not override the browser's built-in
// language negotiation with a server-side assumption like "default to English",
// because the user's browser may already be correctly configured for their
// preferred language.
func UIHandler(session *router.Session, w http.ResponseWriter, r *http.Request) int {
	// Determine which language to use for this dashboard session.
	lang := resolveDashboardLanguage(r)

	// Load the dashboard.html asset from the local asset store. We don't care
	// about its size — the entire file must be in memory so we can substitute
	// the language placeholder before writing it to the response.
	uiAsset, _, err := assets.Loader(session.ID, "/assets/dashboard/dashboard.html", assets.StartOfData, assets.EndOfData)
	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	// Replace every occurrence of the language placeholder with the negotiated
	// language code.  dashboard.html currently has two occurrences:
	//   • <html lang="__EGO_LANG__">
	//   • <meta name="ego-lang" content="__EGO_LANG__">
	uiAsset = bytes.ReplaceAll(uiAsset, []byte(langPlaceholder), []byte(lang))

	// Set the Content-Type header to text/html.
	w.Header().Set("Content-Type", "text/html")

	// Write the substituted dashboard.html content to the response writer.
	_, err = w.Write(uiAsset)
	if err != nil {
		return util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), http.StatusInternalServerError)
	}

	return http.StatusOK
}

// resolveDashboardLanguage returns the language code to inject into the dashboard
// page, or "" if no explicit language was requested.
//
// Only the ?lang= query parameter is consulted.  We deliberately do NOT fall back
// to session.Language (which comes from the request's Accept-Language header) or
// to i18n.DefaultLanguage(), because an empty return value tells dashboard.js to
// leave the Accept-Language header unset — letting the browser send its own native
// value on every API call rather than having the server impose a default.
func resolveDashboardLanguage(r *http.Request) string {
	paramLang := r.URL.Query().Get("lang")
	if paramLang == "" {
		paramLang = r.URL.Query().Get("language")
		if paramLang == "" {
			// No preference expressed — return empty so dashboard.js lets the
			// browser's built-in language negotiation take over.
			return ""
		}
	}

	// Pass the value through NegotiateLanguage, which strips region sub-tags
	// (e.g. "fr-CA" → "fr"), lower-cases the result, and confirms that Ego
	// actually has catalog entries for it.  An unsupported code returns "",
	// which has the same "let the browser decide" effect as a missing param.
	return i18n.NegotiateLanguage(paramLang)
}
