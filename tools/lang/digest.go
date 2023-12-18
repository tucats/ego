package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"
)

var md5Digest hash.Hash

// Initialize the MD5 digest.
func initDigest() {
	md5Digest = md5.New()
}

// Given a file name and it's contents, add it to the MD5 digest.
func addFileToDigest(name string, b []byte) error {
	var err error

	// Add the length of the file and the file name.
	_, err = md5Digest.Write([]byte{byte(len(name))})
	if err == nil {
		_, err = md5Digest.Write([]byte(name))
		if err == nil {
			// Add the file contents.
			_, err = md5Digest.Write(b)
		}
	}

	return err
}

// Return the digest value as a base64-encoded string.
func digestValue(path string) string {
	// Clean up the path string to remove any leading slashes and "../" sequences.
	path = strings.TrimPrefix(path, "/")
	for strings.HasPrefix(path, "../") {
		path = strings.TrimPrefix(path, "../")
	}

	// Format the message using a comment withe the cleaned up path and the digest value.
	msg := `// Archive digest value for %s
%s
`

	return fmt.Sprintf(msg, path, hex.EncodeToString(md5Digest.Sum(nil)))
}
