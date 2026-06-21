package authserver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256" // L1: content-derived JWKS kid
	"crypto/x509"
	"encoding/base64"
	"encoding/hex" // L1: hex-encode the key fingerprint
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath" // L2: same-directory temp file for atomic write

	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/errors"
)

// signingKey is the EC private key loaded (or generated) at startup. All JWT
// signing operations use this key. The public half is published via the JWKS
// endpoint so that Resource Servers can verify tokens without this private key.
var signingKey *ecdsa.PrivateKey

// jwksJSON is the pre-built JSON bytes for the /.well-known/jwks.json response.
// It is computed once from signingKey when the AS is initialized.
var jwksJSON []byte

// loadOrGenerateKey attempts to read an EC private key from the PEM file at
// keyFile. If the file does not exist it generates a new P-256 key pair and
// writes it to keyFile so the same key is reused on subsequent restarts.
//
// The function also pre-computes the JWKS JSON that will be served from the
// /.well-known/jwks.json endpoint.
func loadOrGenerateKey(keyFile string) error {
	// Try to read an existing key from disk.
	pemData, err := os.ReadFile(keyFile)
	if err == nil {
		// File exists — verify permissions before using its contents.  A PEM
		// private key readable by other users on the host is a security risk.
		if permErr := ensureFilePermissions(keyFile); permErr != nil {
			return permErr
		}

		// Decode and parse it.
		block, _ := pem.Decode(pemData)
		if block == nil {
			return errors.New(errors.ErrOAuthKeyNoPEM).Context(keyFile)
		}

		key, parseErr := x509.ParseECPrivateKey(block.Bytes)
		if parseErr != nil {
			return errors.New(errors.ErrOAuthKeyParse).Context(fmt.Sprintf("%s: %v", keyFile, parseErr))
		}

		signingKey = key

		ui.Log(ui.ServerLogger, "oauth.as.key.loaded", ui.A{"path": keyFile})
	} else {
		// File missing or unreadable — generate a fresh P-256 key pair.
		//
		// P-256 (also called secp256r1 or prime256v1) is chosen because:
		//   • Go's standard library has full support (crypto/ecdsa + crypto/elliptic)
		//   • ES256 is the most widely supported ECDSA algorithm in OAuth2 client libraries
		//   • The 32-byte coordinates produce a compact JWKS document
		key, genErr := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if genErr != nil {
			return errors.New(errors.ErrOAuthKeyGenerate).Context(genErr.Error())
		}

		// Serialize the private key to DER (binary) and then wrap it in PEM.
		der, marshalErr := x509.MarshalECPrivateKey(key)
		if marshalErr != nil {
			return errors.New(errors.ErrOAuthKeyMarshal).Context(marshalErr.Error())
		}

		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: der,
		})

		// L2 — atomic key file write via temp-file rename.
		//
		// Why not os.WriteFile directly?
		//
		//   os.WriteFile(path, data, mode) opens the file with O_TRUNC, writes,
		//   then closes.  If the process is killed between truncation and the final
		//   close, the key file on disk is empty (or partially written).  On the
		//   next restart, pem.Decode returns nil and the server fails to start.
		//   Recovering requires manual deletion of the corrupt file — an outage
		//   that is hard to diagnose.
		//
		// How the atomic write works:
		//
		//   1. Create a temp file in the SAME directory as keyFile.  Same directory
		//      is mandatory: os.Rename is only atomic when source and destination
		//      are on the same file system.  Cross-device renames fall back to
		//      copy-then-delete, which is NOT atomic.
		//   2. Write the PEM bytes and close the temp file.
		//   3. chmod to 0600 BEFORE the rename so the file is never world-readable,
		//      even for the brief instant between creation and the permission set.
		//      (os.CreateTemp already uses 0600, but we set it explicitly.)
		//   4. os.Rename atomically replaces keyFile with the temp file.  If the
		//      process dies here the old key (if any) is still intact; if it dies
		//      after, the new key is complete.
		//   5. defer os.Remove(tmpPath) is a no-op after a successful rename
		//      because the temp-file path no longer exists — but it cleans up on
		//      any error return before the rename.
		tmpFile, tmpErr := os.CreateTemp(filepath.Dir(keyFile), "oauth-key-*.pem.tmp")
		if tmpErr != nil {
			return errors.New(errors.ErrOAuthKeyWrite).Context(
				fmt.Sprintf("%s (temp create): %v", keyFile, tmpErr))
		}

		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath) // no-op after successful rename; cleans up on error

		if _, writeErr := tmpFile.Write(pemBytes); writeErr != nil {
			tmpFile.Close()

			return errors.New(errors.ErrOAuthKeyWrite).Context(
				fmt.Sprintf("%s (temp write): %v", keyFile, writeErr))
		}

		if closeErr := tmpFile.Close(); closeErr != nil {
			return errors.New(errors.ErrOAuthKeyWrite).Context(
				fmt.Sprintf("%s (temp close): %v", keyFile, closeErr))
		}

		// Set permissions explicitly even though os.CreateTemp defaults to 0600,
		// so the intent is visible in the code and not dependent on umask.
		if chmodErr := os.Chmod(tmpPath, 0600); chmodErr != nil {
			return errors.New(errors.ErrOAuthKeyWrite).Context(
				fmt.Sprintf("%s (chmod): %v", keyFile, chmodErr))
		}

		// Atomic replace: on POSIX systems os.Rename is guaranteed to be atomic.
		if renameErr := os.Rename(tmpPath, keyFile); renameErr != nil {
			return errors.New(errors.ErrOAuthKeyWrite).Context(
				fmt.Sprintf("%s (rename): %v", keyFile, renameErr))
		}

		signingKey = key

		ui.Log(ui.ServerLogger, "oauth.as.key.generated", ui.A{"path": keyFile})
	}

	// Pre-compute the JWKS JSON from the public half of the key.
	return buildJWKS()
}

// buildJWKS constructs the JSON Web Key Set document from signingKey's public
// key and stores it in jwksJSON. This function is called once at startup; the
// result is served verbatim on every JWKS request, avoiding repeated marshaling.
//
// The JWKS format is defined by RFC 7517. For an EC P-256 public key the
// relevant fields are:
//   - kty: "EC"
//   - crv: "P-256"
//   - x, y: base64url-encoded 32-byte big-endian coordinates
//   - use: "sig" (this key is used for signing, not encryption)
//   - alg: "ES256" (ECDSA with SHA-256)
//   - kid: first 16 hex characters of SHA-256(DER public key) — see L1 note
func buildJWKS() error {
	pub := signingKey.PublicKey

	// L1 — content-derived key identifier (kid).
	//
	// Why NOT use a static "1":
	//
	//   A static kid means every key rotation looks identical to Resource
	//   Servers: before and after the rotation the JWKS endpoint publishes
	//   kid="1".  An RS that has cached the old JWKS (TTL up to 1 hour)
	//   will keep verifying new JWTs against the old key, causing spurious
	//   401 rejections until the RS cache expires.
	//
	//   With a content-derived kid, the new key has a different kid value.
	//   RS libraries that support key rotation (which is every major OAuth2
	//   client library) detect the new kid in the JWT header, see it is absent
	//   from their cached JWKS, and immediately re-fetch — resulting in zero
	//   failed requests after a rotation.
	//
	// How the kid is computed:
	//
	//   1. Marshal the public key to the standard DER/PKIX format.  This is the
	//      same encoding used when publishing keys in a certificate, so it is
	//      canonical and includes the key type and curve metadata.
	//   2. SHA-256 hash the DER bytes.  SHA-256 is collision-resistant:
	//      two different P-256 keys will produce different hashes with
	//      overwhelming probability.
	//   3. Take the first 8 bytes (64 bits) and hex-encode them → 16 hex chars.
	//      8 bytes provides ample uniqueness for key identification — it is 2^64
	//      values.  More bytes add length without any practical benefit.
	//
	// The kid is stable across server restarts as long as the same key file is
	// in use.  It changes automatically when the key is regenerated.
	derPub, derErr := x509.MarshalPKIXPublicKey(&pub)
	if derErr != nil {
		return errors.New(errors.ErrJWKSMarshal).Context(
			"could not encode public key for kid derivation: " + derErr.Error())
	}

	fingerprint := sha256.Sum256(derPub)
	kid := hex.EncodeToString(fingerprint[:8]) // 8 bytes → 16 hex chars

	// The EC coordinates must be padded to exactly 32 bytes (256 bits / 8) for P-256.
	// Big.Int.Bytes() drops leading zeros, so we pad with a fixed-size buffer.
	const coordLen = 32

	xBytes := make([]byte, coordLen)
	yBytes := make([]byte, coordLen)

	pub.X.FillBytes(xBytes)
	pub.Y.FillBytes(yBytes)

	// base64url encoding (no padding) is required by the JWKS specification.
	xB64 := base64.RawURLEncoding.EncodeToString(xBytes)
	yB64 := base64.RawURLEncoding.EncodeToString(yBytes)

	// JWKS document structure — a single-key set for simplicity.
	// Resource Servers iterate over the "keys" array and match by "kid".
	keySet := map[string]any{
		"keys": []map[string]any{
			{
				"kty": "EC",
				"crv": "P-256",
				"use": "sig",
				"alg": "ES256",
				"kid": kid,
				"x":   xB64,
				"y":   yB64,
			},
		},
	}

	b, err := json.Marshal(keySet)
	if err != nil {
		return errors.New(errors.ErrJWKSMarshal).Context(err.Error())
	}

	jwksJSON = b

	return nil
}
