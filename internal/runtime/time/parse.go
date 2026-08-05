package time

import (
	"strings"
	"sync"
	"time"

	// This import has no name and no uses below; the underscore tells Go
	// "load this package for its side effects only". The side effect here is
	// that time/tzdata embeds a copy of the IANA timezone database into the
	// Ego executable, which Go falls back on when the host has no timezone
	// database of its own. Slim server and container images frequently omit
	// one, and without this import time.LoadLocation("America/New_York")
	// would fail there -- meaning ego.runtime.timezone could not be honored
	// in exactly the deployments TIME-1 was reported from. The cost is about
	// 450KB of binary size; the database is only consulted if the host has
	// nothing better.
	_ "time/tzdata"

	"github.com/araddon/dateparse"
	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/language/data"
	"github.com/tucats/ego/internal/language/symbols"
)

// The location cache. Resolving a timezone name into a *time.Location means
// reading and parsing a zoneinfo file from disk, so we remember the result of
// the last lookup instead of repeating it on every call to ParseAny().
//
// The cache is keyed by the configuration *string* rather than just holding a
// single location, so that if the setting changes while the program is running
// (an Ego test can do this with profile.Set(), and the CLI's --set option can
// do it for one run) the next call notices the new name and re-resolves it.
//
// A sync.Mutex is Go's simplest lock: only one goroutine at a time may hold
// it. Ego programs can run goroutines, and several of them could call
// ParseAny() at the same moment, so every read and write of the two cache
// variables happens while the lock is held. Without the lock, two goroutines
// writing the cache simultaneously would be a data race.
var (
	locationLock  sync.Mutex
	locationName  string
	locationValue *time.Location
)

// defaultLocation returns the *time.Location that should be used as the
// reference point for interpreting a bare timezone abbreviation, along with an
// error if the configured timezone name cannot be resolved.
//
// Why a "reference point" is needed at all: an abbreviation like "EST" is just
// three letters. It carries no numeric offset from UTC, and the abbreviations
// are not unique across the world -- "CST" is US Central Standard Time
// (-06:00), China Standard Time (+08:00), and Cuba Standard Time (-05:00),
// depending on who is writing it. Go can therefore only turn an abbreviation
// into a real offset by looking it up in the zone table of some particular
// location, and something has to choose that location.
//
// The choice is made in this order:
//
//  1. The ego.runtime.timezone configuration setting, when it names a specific
//     zone. This is the deterministic option: the answer no longer depends on
//     the machine or container the program happens to run on.
//
//  2. The host's own local timezone, when the setting is missing or is the
//     word "local" (which is the value seeded into a new configuration). This
//     is the "best guess from the user's own locale" case, described further
//     below.
//
// The returned *time.Location is never nil when the error is nil.
func defaultLocation() (*time.Location, error) {
	// Read the configured name. settings.Get() returns an empty string for a
	// setting that has never been given a value, which we treat exactly like
	// the explicit word "local" -- see the comment on defs.LocalTimeZone.
	name := strings.TrimSpace(settings.Get(defs.RuntimeTimeZoneSetting))
	if name == "" {
		name = defs.LocalTimeZone
	}

	// Lock the cache for the rest of this function. "defer" schedules the
	// Unlock call to run when the function returns, by any path -- including
	// the error returns below -- so the lock can never be left held.
	locationLock.Lock()
	defer locationLock.Unlock()

	// A cache hit: the setting has not changed since the last lookup, so the
	// location we resolved then is still the right answer.
	if locationValue != nil && locationName == name {
		return locationValue, nil
	}

	var (
		loc *time.Location
		err error
	)

	// Go's time.LoadLocation() understands the exact spellings "Local" and
	// "UTC" as well as IANA database names, but it is case-sensitive, so a
	// configuration value of "local" or "utc" would fail. Handle those two
	// words ourselves, case-insensitively, and hand everything else to
	// LoadLocation.
	switch strings.ToLower(name) {
	case defs.LocalTimeZone:
		// time.Local is Go's representation of the host's configured timezone.
		// This is where the "best guess based on the user's own locale" comes
		// from. Go builds time.Local at startup from, in order:
		//
		//   - the TZ environment variable, if it is set (TZ="America/Denver"),
		//   - otherwise the host's /etc/localtime file, which on macOS and
		//     Linux is a symlink to the zone the machine is configured for,
		//   - otherwise UTC, if neither source says anything.
		//
		// That last fallback is the important caveat: a bare container image
		// or a minimal server install usually has no TZ and no /etc/localtime,
		// so its "local" timezone is UTC, and no abbreviation other than "UTC"
		// or "GMT" will resolve there. There is no richer source to consult --
		// Go exposes no other locale information, and in any case a language
		// or country locale does not determine a timezone (the United States
		// spans six of them). A deployment that needs a specific answer must
		// set ego.runtime.timezone explicitly.
		loc = time.Local

	case strings.ToLower(defs.UTCTimeZone):
		loc = time.UTC

	default:
		loc, err = time.LoadLocation(name)
		if err != nil {
			// Report the bad configuration value rather than Go's lower-level
			// "unknown time zone" text, and do not poison the cache with it.
			return nil, errors.ErrInvalidTimeZone.Context(name)
		}
	}

	// Record the successful lookup for next time.
	locationName = name
	locationValue = loc

	return loc, nil
}

// Parse an arbitrary string value into a native Go datetime value. Uses the dateparse
// package which first scans the string to determine the appropriate Go date format string,
// and then uses that string to do the conversion.
//
// This implements the Ego time.ParseAny() function.
//
// The work is done in two passes so that a timestamp which names no timezone
// keeps its historical meaning while one that names a timezone *abbreviation*
// becomes reproducible (TIME-1):
//
//	"Dec 7, 1959"                       -> 1959-12-07 00:00:00 +0000 UTC
//	"December 7, 1959 10:35am EST"      -> 1959-12-07 10:35:00 -0500 EST
//	"2024-01-15T10:00:00-08:00"         -> 2024-01-15 10:00:00 -0800
//
// The first has no zone information at all, and is read as UTC. The third
// states its offset numerically, so there is nothing to resolve. Only the
// second needs a reference location, and that is what the second pass supplies.
func Parse(s *symbols.SymbolTable, args data.List) (any, error) {
	value := data.String(args.Get(0))

	// Pass one: parse relative to UTC. dateparse.ParseIn() is the
	// location-aware form of ParseAny(); passing time.UTC pins down both of
	// the places the location matters:
	//
	//   - a string with no zone information is read as UTC, which is what
	//     ParseAny() already did (Go's time.Parse() defaults to UTC), so
	//     existing programs see no change; and
	//   - a bare abbreviation is left *unresolved*, keeping its name but
	//     taking an offset of zero.
	//
	// The bug this replaces was that plain ParseAny() resolved abbreviations
	// against time.Local implicitly, so the same input produced a different
	// offset on a developer laptop than in a UTC container, with nothing in
	// the result to indicate it had happened.
	t, e := dateparse.ParseIn(value, time.UTC)
	if e != nil {
		e = errors.New(e).In("ParseAny")

		return data.NewList(nil, e), e
	}

	// Zone() reports the abbreviation and offset that the parsed time carries.
	// Go's assignment of the two return values here uses "_" to discard the
	// offset, which we do not need.
	//
	// The name tells us which of the three cases above we are in:
	//
	//   - ""    : the string gave a numeric offset only (case three), or none
	//             at all in a format Go treats as offset-only. Nothing to do.
	//   - "UTC" : the string named no zone (case one) or explicitly said UTC
	//             or "Z". UTC is already the correct answer.
	//   - other : the string carried an abbreviation (case two) that UTC could
	//             not resolve, which is exactly the case needing pass two.
	zoneName, _ := t.Zone()
	if zoneName == "" || zoneName == defs.UTCTimeZone {
		return data.NewList(t, nil), nil
	}

	// Pass two: an abbreviation is present, so look it up in the configured
	// reference location.
	loc, err := defaultLocation()
	if err != nil {
		// The configured timezone name is not a zone Go can load. That is a
		// configuration error the user needs to see, not something to paper
		// over with a silently wrong offset. errors.New() recognizes that this
		// is already an Ego error and clones it rather than double-wrapping.
		err = errors.New(err).In("ParseAny")

		return data.NewList(nil, err), err
	}

	// If the reference location is UTC there is nothing pass two could add;
	// the answer would be identical to what pass one already produced.
	if loc == time.UTC {
		return data.NewList(t, nil), nil
	}

	// Re-parse against the reference location. time.ParseInLocation (which
	// ParseIn uses underneath) searches that location's zone table for the
	// abbreviation: parsing "EST" against America/New_York finds the -05:00
	// winter zone and applies it.
	//
	// If the abbreviation is not one this location uses at all -- "JST"
	// against America/New_York, say -- Go leaves the offset at zero and keeps
	// the name, so the result is the same as pass one's. That is deliberately
	// not treated as an error: it matches what the function has always
	// returned for an unrecognized abbreviation, and callers that need
	// certainty should supply a numeric offset in the input instead.
	//
	// An error here would mean the string parsed a moment ago and now does
	// not, which should not be possible; keeping pass one's result is the
	// safe response if it ever happens.
	if located, err := dateparse.ParseIn(value, loc); err == nil {
		t = located
	}

	return data.NewList(t, nil), nil
}
