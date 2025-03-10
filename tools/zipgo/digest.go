package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
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
func digestValue(path string) string {
	// Clean up the path string to remove any leading slashes and "../" sequences.
	path = strings.ReplaceAll(strings.TrimPrefix(path, "/"), "\n", "")
	for strings.HasPrefix(path, "../") {
		path = strings.TrimPrefix(path, "../")
	}

	// Format the message using a comment with the cleaned up path and the digest value.
	msg := "// %s digest: %s\n"

	return fmt.Sprintf(msg, path, hex.EncodeToString(md5Digest.Sum(nil)))
}
