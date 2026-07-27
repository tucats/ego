package util

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"strconv"
	"strings"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
)

// DefaultCompressionThreshold is the payload size (in bytes) at or above which a
// response is worth compressing, when the configuration does not specify a value.
//
// Why not compress everything? Gzip adds an 18-byte envelope of its own, costs CPU
// time on both ends, and a payload small enough to fit in one or two network packets
// arrives just as fast either way. 4096 is a conservative point where the savings
// reliably exceed the overhead.
const DefaultCompressionThreshold = 4096

// gzipEncodingName is the token used in the HTTP Accept-Encoding and Content-Encoding
// headers to name the gzip compression format. HTTP header tokens are case-insensitive,
// so all comparisons against this value must be done case-insensitively.
const gzipEncodingName = "gzip"

// ResponseInfo carries the handful of per-request facts that the response writers need.
// All three come from the request's Session, and grouping them keeps the writer
// functions to a short argument list instead of a run of loose ints and bools that
// would be easy to transpose by accident.
//
// The util package cannot refer to the router's Session type directly -- the router
// imports util, so pointing back the other way would be an import cycle -- which is why
// these values are copied into a small struct here. Handlers get one by calling the
// Session's Response() method.
type ResponseInfo struct {
	// SessionID identifies the request in the server log. Log entries carrying it are
	// prefixed with "[id]", which is what makes a line attributable when many requests
	// are being served at once.
	SessionID int

	// AcceptsGzip is true when the client said it can decode a compressed body.
	AcceptsGzip bool

	// Length accumulates the number of bytes actually sent on the network. May be nil
	// for callers that do not track a response size.
	Length *int
}

// CompressionThreshold returns the minimum response payload size (in bytes) that will
// trigger gzip compression of a response body.
//
// The value comes from the ego.server.compression.threshold configuration setting.
// Three cases are handled:
//
//	unset      the DefaultCompressionThreshold is used
//	zero       compression is disabled entirely (the caller sees a threshold of 0
//	           meaning "never", which WriteMaybeCompressed special-cases)
//	other      the configured number of bytes is used
//
// Note that we deliberately read the setting as a string rather than calling
// settings.GetInt(). GetInt() returns 0 for a setting that was never configured,
// which would make "not configured" indistinguishable from an explicit "0" -- and
// those two cases need to mean opposite things here.
func CompressionThreshold() int {
	text := strings.TrimSpace(settings.Get(defs.ServerCompressionThresholdSetting))
	if text == "" {
		return DefaultCompressionThreshold
	}

	value, err := strconv.Atoi(text)
	if err != nil || value < 0 {
		// A malformed or negative setting is not worth failing a request over;
		// fall back to the default and carry on.
		return DefaultCompressionThreshold
	}

	return value
}

// AcceptsGzip reports whether the client that sent this request is willing to receive
// a gzip-compressed response body.
//
// Clients advertise this using the Accept-Encoding request header, which holds a
// comma-separated list of encoding names, each optionally carrying a "quality value"
// between 0 and 1 that expresses preference:
//
//	Accept-Encoding: gzip                 -- gzip is fine
//	Accept-Encoding: gzip, deflate        -- either is fine
//	Accept-Encoding: gzip;q=0.5, br       -- both fine, br preferred
//	Accept-Encoding: *                    -- anything is fine
//	Accept-Encoding: gzip;q=0             -- gzip is explicitly REFUSED
//	Accept-Encoding: identity             -- send it uncompressed, please
//	(header absent)                       -- assume uncompressed
//
// A quality value of exactly zero is the standard way of saying "not this one", so
// "gzip;q=0" must be honored as a refusal even though the word "gzip" appears in the
// header. An explicit refusal of gzip also overrides a wildcard "*" appearing in the
// same header, which is why this function scans the whole list before deciding.
func AcceptsGzip(r *http.Request) bool {
	if r == nil {
		return false
	}

	// A missing header means the client said nothing about compression, so the safe
	// interpretation is that it wants the body as-is.
	header := r.Header.Get("Accept-Encoding")
	if header == "" {
		return false
	}

	var (
		wildcardAccepted bool
		gzipSeen         bool
		gzipAccepted     bool
	)

	// The header is a comma-separated list. Each element is an encoding name that may
	// be followed by semicolon-separated parameters, of which we care about only "q".
	for _, element := range strings.Split(header, ",") {
		parts := strings.Split(strings.TrimSpace(element), ";")

		name := strings.ToLower(strings.TrimSpace(parts[0]))
		if name == "" {
			continue
		}

		// Assume the encoding is acceptable unless an explicit q=0 says otherwise.
		// The HTTP specification's default quality value is 1.0.
		acceptable := true

		for _, parameter := range parts[1:] {
			parameter = strings.ToLower(strings.TrimSpace(parameter))
			if !strings.HasPrefix(parameter, "q=") {
				continue
			}

			// A q value that we cannot parse is treated as "acceptable", matching the
			// permissive behavior of common HTTP servers.
			if quality, err := strconv.ParseFloat(strings.TrimPrefix(parameter, "q="), 64); err == nil {
				acceptable = quality > 0.0
			}
		}

		switch name {
		case gzipEncodingName:
			gzipSeen = true
			gzipAccepted = acceptable

		case "*":
			wildcardAccepted = acceptable
		}
	}

	// An explicit mention of gzip -- accepting or refusing it -- always wins over the
	// wildcard, because it is the more specific statement about this exact encoding.
	if gzipSeen {
		return gzipAccepted
	}

	return wildcardAccepted
}

// WriteMaybeCompressed writes a complete HTTP response body, transparently gzipping it
// when doing so is both permitted by the client and worthwhile for a payload this size.
// It returns true when the body was actually compressed.
//
// The "info" argument supplies whether the client can decode a compressed body (worked
// out once per request by the router), the session ID used for logging, and the counter
// that accumulates the number of bytes actually placed on the network. That count is the
// compressed size when compression was applied, which keeps the server's request log
// reporting what really crossed the wire rather than the pre-compression size -- the
// number an administrator investigating bandwidth actually wants.
//
// The caller passes the response body as a finished slice of bytes rather than streaming
// it, because the decision to compress depends on knowing the total size up front.
//
// An empty contentType leaves the Content-Type header alone, for callers that have
// already set it themselves.
//
// Every compressed response is reported to the REST logger with its before and after
// sizes. Doing that here rather than in each caller means the saving is visible for all
// compressed responses -- table rowsets, DSN lists, and server logs alike -- from a
// single place that cannot fall out of step with the compression decision itself.
//
// Ordering matters in the code below and is a common source of HTTP bugs: every header
// must be set BEFORE the call to WriteHeader(), because WriteHeader() is the moment the
// status line and headers are flushed to the client. Any header set afterwards is
// silently discarded. For the same reason, a caller must not have called WriteHeader()
// before reaching this function.
func WriteMaybeCompressed(
	w http.ResponseWriter,
	info ResponseInfo,
	status int,
	contentType string,
	body []byte,
) (bool, error) {
	threshold := CompressionThreshold()

	// Decide whether to compress. All of these conditions must hold:
	//
	//   - compression is not switched off by configuration (threshold of zero),
	//   - the payload is at least as large as the threshold,
	//   - the client told us it can decode gzip.
	compress := threshold > 0 && len(body) >= threshold && info.AcceptsGzip

	payload := body

	if compress {
		compressed, err := gzipBytes(body)
		if err != nil {
			// Compression failed for some unexpected reason. Rather than failing the
			// whole request, fall back to sending the body uncompressed -- the client
			// gets correct data either way.
			compress = false
		} else if len(compressed) >= len(body) {
			// Gzip does not shrink every input. Already-compressed or high-entropy
			// data can come out slightly larger, in which case sending the original
			// is both smaller and cheaper for the client to process.
			compress = false
		} else {
			payload = compressed
		}
	}

	if contentType != "" {
		w.Header().Set(defs.ContentTypeHeader, contentType)
	}

	if compress {
		// Content-Encoding tells the client how to decode the bytes that follow. Without
		// it, the client would hand raw gzip bytes to a JSON parser and fail.
		w.Header().Set("Content-Encoding", gzipEncodingName)

		// Vary tells any caching proxy between us and the client that this response
		// depends on the request's Accept-Encoding header, so a compressed response is
		// never replayed to a client that cannot decode it. Omitting this is a classic
		// way to serve unreadable pages to older clients sitting behind a shared cache.
		w.Header().Set("Vary", "Accept-Encoding")
	}

	// Note that we never set Content-Length ourselves. Go computes it automatically for
	// small responses and switches to chunked transfer encoding for larger ones. Setting
	// it by hand to the uncompressed size would truncate or hang the response.
	w.WriteHeader(status)

	count, err := w.Write(payload)
	if info.Length != nil {
		*info.Length += count
	}

	if compress {
		logCompression(info.SessionID, len(body), count)
	}

	return compress, err
}

// logCompression records how much a compressed response saved, for the REST logger.
//
// Reporting the saving as a percentage rather than only two byte counts makes it
// obvious at a glance whether compression is earning its keep on a given endpoint. The
// session ID is included so the entry can be tied to a specific request: the logger
// renders it as a "[id]" prefix, which matters on a busy server where many requests
// interleave their log output.
func logCompression(sessionID, originalSize, sentSize int) {
	if !ui.IsActive(ui.RestLogger) {
		return
	}

	// Guard against a divide-by-zero on an empty body. An empty payload can never
	// really be compressed, but costs nothing to handle safely.
	percent := 0
	if originalSize > 0 {
		percent = 100 - (sentSize * 100 / originalSize)
	}

	ui.Log(ui.RestLogger, "rest.response.compressed", ui.A{
		"session": sessionID,
		"size":    originalSize,
		"sent":    sentSize,
		"percent": percent})
}

// gzipBytes compresses a block of bytes into a complete, self-contained gzip stream.
//
// gzip.Writer streams its output to some destination writer; here that destination is
// an in-memory buffer, so the result can be measured before deciding to use it. The
// Close() call is essential and is not merely cleanup: it flushes the final block and
// writes the gzip trailer (the checksum and length), without which the stream is
// truncated and every decompressor will report a corrupt file.
func gzipBytes(body []byte) ([]byte, error) {
	var buffer bytes.Buffer

	// BestSpeed rather than the default level: server log payloads are highly repetitive
	// text that already compresses roughly ten to one at the fastest setting, and the
	// extra CPU spent chasing a slightly smaller payload would be paid on every request.
	writer, err := gzip.NewWriterLevel(&buffer, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}

	if _, err := writer.Write(body); err != nil {
		// Close the writer even on a failed write so its internal resources are released.
		_ = writer.Close()

		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
