package authserver

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/tucats/ego/app-cli/ui"
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
		// File exists — decode and parse it.
		block, _ := pem.Decode(pemData)
		if block == nil {
			return fmt.Errorf("key file %s contains no PEM block", keyFile)
		}

		key, parseErr := x509.ParseECPrivateKey(block.Bytes)
		if parseErr != nil {
			return fmt.Errorf("parsing EC key from %s: %w", keyFile, parseErr)
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
			return fmt.Errorf("generating EC key: %w", genErr)
		}

		// Serialize the private key to DER (binary) and then wrap it in PEM.
		der, marshalErr := x509.MarshalECPrivateKey(key)
		if marshalErr != nil {
			return fmt.Errorf("marshaling EC key: %w", marshalErr)
		}

		pemBytes := pem.EncodeToMemory(&pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: der,
		})

		// Write the PEM file. 0600 means only the owning user can read/write it,
		// which is appropriate for a private key.
		if writeErr := os.WriteFile(keyFile, pemBytes, 0600); writeErr != nil {
			return fmt.Errorf("writing key to %s: %w", keyFile, writeErr)
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
//   - kid: "1" (a stable key ID; single key so a static value is fine)
func buildJWKS() error {
	pub := signingKey.PublicKey

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
				"kid": "1",
				"x":   xB64,
				"y":   yB64,
			},
		},
	}

	b, err := json.Marshal(keySet)
	if err != nil {
		return fmt.Errorf("marshaling JWKS: %w", err)
	}

	jwksJSON = b

	return nil
}
