package util

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
)

// This file is the one place Ego decides what a bare timezone abbreviation
// means. Both the Ego runtime's time.ParseAny() function and the database
// table layer's timestamp coercion go through it, so the two cannot drift
// apart or disagree. See docs/issues/resolved/TIME-1.md and
// docs/issues/TIME-2.md.

// The location cache. Resolving a timezone name into a *time.Location means
// reading and parsing a zoneinfo file from disk, so we remember the result of
// the last lookup instead of repeating it on every parse.
//
// The cache is keyed by the configuration *string* rather than just holding a
// single location, so that if the setting changes while the program is running
// (an Ego test can do this with profile.Set(), and the CLI's --set option can
// do it for one run) the next call notices the new name and re-resolves it.
//
// A sync.Mutex is Go's simplest lock: only one goroutine at a time may hold
// it. An Ego program can run goroutines and a server handles requests
// concurrently, so several callers could arrive here at the same moment. Every
// read and write of the two cache variables happens while the lock is held;
// without it, two goroutines writing the cache simultaneously would be a data
// race.
var (
	locationLock  sync.Mutex
	locationName  string
	locationValue *time.Location
)

// universalZeroZones are the abbreviations that mean an offset of exactly zero
// everywhere on earth, so they need no reference location to be understood.
//
// They matter because Go cannot resolve them against, say, America/New_York --
// "GMT" is not in that location's zone table -- yet they are not ambiguous the
// way "EST" or "CST" are. Without this list, StrictParseTimestamp() below would
// reject a perfectly clear timestamp.
//
// A Go map used as a set: the keys are what matter and the empty struct{}
// values occupy no memory.
var universalZeroZones = map[string]struct{}{
	"UTC": {},
	"GMT": {},
	"UT":  {},
	"Z":   {},
}

// DefaultLocation returns the *time.Location that should be used as the
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
func DefaultLocation() (*time.Location, error) {
	// Read the configured name. settings.Get() returns an empty string for a
	// setting that has never been given a value, which we treat exactly like
	// the explicit word "local" -- see the comment on defs.LocalTimeZone.
	name := strings.TrimSpace(settings.Get(defs.RuntimeTimeZoneSetting))
	if name == "" {
		name = defs.LocalTimeZone
	}

	// Lock the cache for the rest of this function. "defer" schedules the
	// Unlock call to run when the function returns, by any path -- including
	// the error return below -- so the lock can never be left held.
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

// ParseTimestamp converts an arbitrary timestamp string into a time.Time,
// resolving any bare zone abbreviation against the configured reference
// location (see DefaultLocation).
//
// An abbreviation the reference location does not recognize is *not* an error
// here: the name is kept and the offset stays zero. That is what Ego's
// time.ParseAny() has always returned for an unresolvable abbreviation, and
// changing it would break existing Ego programs that tolerate the zero offset.
// Callers that persist the result should use StrictParseTimestamp instead --
// see the comment there for why the tradeoff comes out differently.
//
// The work is done in two passes so that a timestamp which names no timezone
// keeps its historical meaning while one that names an abbreviation becomes
// reproducible:
//
//	"Dec 7, 1959"                       -> 1959-12-07 00:00:00 +0000 UTC
//	"December 7, 1959 10:35am EST"      -> 1959-12-07 10:35:00 -0500 EST
//	"2024-01-15T10:00:00-08:00"         -> 2024-01-15 10:00:00 -0800
//
// The first has no zone information at all, and is read as UTC. The third
// states its offset numerically, so there is nothing to resolve. Only the
// second needs a reference location.
func ParseTimestamp(value string) (time.Time, error) {
	parsed, _, err := parseTimestamp(value)

	return parsed, err
}

// StrictParseTimestamp is ParseTimestamp with one added rule: a zone
// abbreviation the reference location cannot resolve is rejected with
// ErrAmbiguousTimeZone rather than silently given a zero offset.
//
// Use this whenever the parsed value is going to be *stored*. A wrong offset
// in a running program is a transient bug; a wrong offset written to a
// database column becomes the record, reads back cleanly forever after, and
// cannot be repaired without knowing what timezone the process was configured
// with at the moment of the write. Rejecting the input is recoverable for the
// caller; accepting it silently is not (TIME-2).
func StrictParseTimestamp(value string) (time.Time, error) {
	parsed, resolved, err := parseTimestamp(value)
	if err != nil {
		return parsed, err
	}

	if !resolved {
		// Report the abbreviation that could not be resolved, so the message
		// names the actual problem rather than the whole timestamp.
		zoneName, _ := parsed.Zone()

		return time.Time{}, errors.ErrAmbiguousTimeZone.Context(zoneName)
	}

	return parsed, nil
}

// parseTimestamp does the shared work of the two exported parse functions. It
// returns the parsed time, and whether any zone abbreviation in the input was
// actually resolved to a real offset -- which is the fact the strict caller
// needs and the lenient one ignores.
//
// A timestamp with no abbreviation at all counts as resolved: there was
// nothing to resolve, and nothing ambiguous about it.
func parseTimestamp(value string) (time.Time, bool, error) {
	// Pass one: parse relative to UTC. dateparse.ParseIn() is the
	// location-aware form of ParseAny(); passing time.UTC pins down both of
	// the places the location matters:
	//
	//   - a string with no zone information is read as UTC, which is what
	//     ParseAny() does (Go's time.Parse() defaults to UTC), so callers
	//     that were using ParseAny see no change; and
	//   - a bare abbreviation is left *unresolved*, keeping its name but
	//     taking an offset of zero.
	//
	// The bug this replaces was that plain ParseAny() resolved abbreviations
	// against time.Local implicitly, so the same input produced a different
	// offset on a developer laptop than in a UTC container, with nothing in
	// the result to indicate it had happened.
	parsed, err := dateparse.ParseIn(value, time.UTC)
	if err != nil {
		return time.Time{}, false, errors.New(err)
	}

	// Zone() reports the abbreviation and offset that the parsed time carries.
	// Go's assignment of the two return values here uses "_" to discard the
	// offset, which we do not need.
	//
	// The name tells us which of the three cases above we are in:
	//
	//   - ""    : the string gave a numeric offset only. Nothing to resolve.
	//   - "UTC" : the string named no zone, or explicitly said UTC or "Z".
	//             UTC is already the correct answer.
	//   - other : the string carried an abbreviation that UTC could not
	//             resolve, which is the case needing pass two.
	zoneName, _ := parsed.Zone()
	if zoneName == "" || zoneName == defs.UTCTimeZone {
		return parsed, true, nil
	}

	// An abbreviation that means zero everywhere needs no reference location,
	// and would never match one anyway -- "GMT" is not in America/New_York's
	// zone table even though its meaning is not in doubt.
	if _, ok := universalZeroZones[strings.ToUpper(zoneName)]; ok {
		return parsed, true, nil
	}

	// Pass two: an abbreviation is present, so look it up in the configured
	// reference location.
	loc, err := DefaultLocation()
	if err != nil {
		// The configured timezone name is not a zone Go can load. That is a
		// configuration error the caller needs to see, not something to paper
		// over with a silently wrong offset.
		return time.Time{}, false, err
	}

	// If the reference location is UTC there is nothing pass two could add;
	// the answer would be identical to what pass one already produced, and
	// UTC's zone table holds no regional abbreviation, so nothing resolves.
	if loc == time.UTC {
		return parsed, false, nil
	}

	// Re-parse against the reference location. time.ParseInLocation (which
	// ParseIn uses underneath) searches that location's zone table for the
	// abbreviation: parsing "EST" against America/New_York finds the -05:00
	// winter zone and applies it.
	//
	// An error here would mean the string parsed a moment ago and now does
	// not, which should not be possible; keeping pass one's result is the
	// safe response if it ever happens.
	located, err := dateparse.ParseIn(value, loc)
	if err != nil {
		return parsed, false, nil
	}

	// Did the abbreviation actually resolve? Go answers this through the
	// location it attached to the result, not through the offset:
	//
	//   - resolved     -> the result's Location() is the reference location
	//                     we passed in, because Go found the abbreviation in
	//                     that location's zone table.
	//   - not resolved -> Go fabricates a fixed zone named after the
	//                     abbreviation with an offset of zero, so Location()
	//                     is that throwaway zone instead.
	//
	// Testing the offset instead of the location would be wrong: "WET" in
	// Europe/Lisbon legitimately *is* offset zero in winter, and would be
	// misreported as unresolved.
	return located, located.Location() == loc, nil
}
