package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
)

// checksum accumulates a hash over everything that goes into the archive. It
// is only created when the "--digest" option is used.
//
// SHA-256 is used rather than something shorter such as MD5. Nothing here is
// security sensitive -- this only answers "did the source tree change?" -- but
// MD5 is a broken hash, and static analysis tools quite reasonably flag every
// use of it, so there is no reason to prefer it when a sound alternative is
// equally convenient.
//
// The point of the digest is to avoid rewriting the archive when nothing has
// actually changed. Rewriting it would give the file a new modification time,
// which would make Go's build cache consider every package that embeds it out
// of date, forcing an unnecessary rebuild on every single build.
var checksum hash.Hash

// initDigest prepares the digest accumulator.
func initDigest() {
	checksum = sha256.New()
}

// addFileToDigest folds one archive entry -- its name and its contents --
// into the running checksum.
//
// The name's length is written before the name itself so that the digest can
// tell apart two different sets of files that happen to produce the same run
// of bytes when their names are simply concatenated. Without a length prefix,
// files named "ab" and "c" would digest identically to files named "a" and
// "bc".
//
// The length is written as a fixed 8 bytes. The previous version of this code
// wrote it as a single byte, which silently wrapped around for any name 256
// characters or longer, weakening the very property the prefix exists to
// provide.
func addFileToDigest(name string, data []byte) {
	addLengthToDigest(len(name))
	checksum.Write([]byte(name))
	addLengthToDigest(len(data))
	checksum.Write(data)
}

// addStringToDigest folds an arbitrary string -- not a file, but something
// like the list of omitted file names -- into the running checksum, again
// with a length prefix so it cannot be confused with adjacent data.
func addStringToDigest(text string) {
	addLengthToDigest(len(text))
	checksum.Write([]byte(text))
}

// addLengthToDigest writes a length into the digest as 8 bytes in a fixed
// byte order. A fixed order is used, rather than the machine's native one, so
// that a big-endian and a little-endian machine compute the same digest from
// the same source tree.
func addLengthToDigest(length int) {
	var buffer [8]byte

	// The conversion from int to uint64 is safe here because every value that
	// reaches this function comes from Go's len operator, which never returns
	// a negative number. Static analysis cannot know that on its own, so it
	// is stated explicitly.
	binary.BigEndian.PutUint64(buffer[:], uint64(length)) //nolint:gosec // length comes from len(), never negative
	checksum.Write(buffer[:])
}

// digestValue returns the finished checksum as a printable hexadecimal
// string. This string is stored in the archive itself, as the ZIP file's
// trailing comment field, and is compared against on the next run to decide
// whether the archive needs to be rebuilt.
func digestValue() string {
	return hex.EncodeToString(checksum.Sum(nil))
}
