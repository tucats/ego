package errors

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/ego/internal/i18n"
)

// Error formats this Ego Error as a string for human consumption, in the
// process-wide default language (see i18n.DefaultLanguage). This is the
// method that satisfies Go's built-in "error" interface, so it is called
// automatically whenever this *Error is passed to fmt.Println, used with
// the "%v"/"%s" formatting verbs, or compared with .Error() by other code.
//
// Command-line code, which only ever has one user and one language for
// the whole process, should keep using this method exactly as before.
// REST server code that needs to render the SAME underlying error in
// whatever language a specific caller asked for (for example, via an
// Accept-Language header) should call Localize instead — see below.
func (e *Error) Error() string {
	return e.errorText(i18n.DefaultLanguage())
}

// Localize formats this Ego Error exactly like Error() does, except it
// always renders the message in the given lang ("en", "fr", "es", ...)
// instead of the process-wide default language.
//
// This matters for the REST server, where many requests from callers who
// each prefer a different language can be in flight on different
// goroutines at the same time. Error() reads the shared, process-wide
// i18n.Language setting (by way of i18n.DefaultLanguage), so it always
// describes one error the same way for everybody. Localize, in contrast,
// never reads or writes any shared state — it only uses the lang value it
// was given — so it is safe to call concurrently: one goroutine can call
// someErr.Localize("fr") for one request while, at the very same instant,
// another goroutine calls the very same someErr.Localize("es") for a
// different request, and the two calls cannot interfere with each other.
//
// A typical caller is a REST handler that has a *router.Session (which
// carries the negotiated Session.Language for the current request):
//
//	msg := someErr.Localize(session.Language)
//	util.ErrorResponse(w, session.ID, msg, http.StatusBadRequest)
func (e *Error) Localize(lang string) string {
	return e.errorText(lang)
}

// Localize renders any error in the given lang if it is (or wraps) an Ego
// *Error, falling back to err.Error() for any other error type (which has
// no language-specific rendering to offer). This is a convenience for call
// sites that only have a plain "error" interface value in hand -- typically
// because the error came back from a function signature like "func() error"
// -- rather than the concrete *Error type that Localize's method form
// requires.
//
// A typical caller is a REST handler with a *router.Session in scope:
//
//	if err := doSomething(); err != nil {
//	    util.ErrorResponse(w, session.ID, errors.Localize(err, session.Language), status)
//	}
func Localize(err error, lang string) string {
	if err == nil {
		return ""
	}

	if e, ok := err.(*Error); ok {
		return e.Localize(lang)
	}

	return err.Error()
}

// errorText contains the formatting logic shared by Error() and
// Localize(). It is the same step-by-step construction the original
// Error() method used, except every translation lookup now takes lang as
// an explicit argument (via i18n.ELang / i18n.LLang) instead of implicitly
// reading the process-wide default language (via i18n.E / i18n.L). That
// one change is what lets Error() and Localize() share this single
// implementation while still behaving differently with respect to which
// language they produce.
func (e *Error) errorText(lang string) string {
	var (
		b         strings.Builder
		predicate bool
	)

	if e == nil || e.err == nil {
		return ""
	}

	// Format the underlying error message text. Apply error localizations if available.
	errText := e.err.Error()
	text := i18n.ELang(lang, strings.TrimPrefix(errText, "error."))

	// If we have a location, report that as module or module/line number
	if e.location != nil {
		if predicate {
			b.WriteString(", ")
		}

		if e.location.line > 0 || e.location.column > 0 {
			lineStr := strconv.Itoa(e.location.line)

			if e.location.column > 0 {
				lineStr = lineStr + ":" + strconv.Itoa(e.location.column)
			}

			b.WriteString("at ")

			if len(e.location.name) > 0 {
				b.WriteString(fmt.Sprintf("%s(line %s)", e.location.name, lineStr))
			} else {
				b.WriteString(fmt.Sprintf("line %s", lineStr))
			}

			predicate = true
		} else {
			if e.location.name != "" {
				b.WriteString("in ")
				b.WriteString(e.location.name)

				predicate = true
			}
		}
	}

	// If we have an underlying error, report the string value for that
	if !Nil(e.err) {
		if !e.Is(ErrUserDefined) {
			if predicate {
				b.WriteString(", ")
			}

			msg := strings.TrimPrefix(text, "error.")
			predicate = true

			b.WriteString(msg)
		}
	}

	// If we have additional context, report that
	if e.context != "" {
		if predicate {
			b.WriteString(": ")
		}

		b.WriteString(e.context)
	}

	// If this is part of a chain, format the linked message first. If the
	// message is getting kind of long, add a break to it.
	//
	// Note: this passes lang down to e.next.errorText(lang) (rather than
	// calling e.next.Error()), so a chained/linked error is rendered in
	// the same language as the outer error, instead of silently reverting
	// to the process default language partway through the message.
	if e.next != nil {
		b.WriteString(",\n")

		errorString := i18n.LLang(lang, "Error") + ": "

		b.WriteString(strings.Repeat(" ", len(errorString)))
		b.WriteString(e.next.errorText(lang))
	}

	return b.String()
}
