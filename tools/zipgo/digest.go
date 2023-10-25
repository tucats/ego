package main

import (
	"crypto/md5"
	"encoding/hex"
	"hash"
	"io"
	"os"
)

var md5Digest hash.Hash

// Initialize the MD5 digest.
func initDigest() {
	md5Digest = md5.New()
}

// Given a file name and it's contents, add it to the MD5 digest.
func addFileToDigest(name string, f *os.File) error {
	// Add the length of the file and the file name.
	md5Digest.Write([]byte{byte(len(name))})
	md5Digest.Write([]byte(name))

	// Add the file contents.
	_, err := io.Copy(md5Digest, f)

	return err
}

// Return the digest value as a base64-encoded string.
func digestValue() string {
	return hex.EncodeToString(md5Digest.Sum(nil))
}
