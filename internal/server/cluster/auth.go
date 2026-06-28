package cluster

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/tucats/ego/internal/cli/settings"
	"github.com/tucats/ego/internal/defs"
)

const prefix = "Bearer "

// clusterToken computes a deterministic bearer token that any node in the
// cluster can independently verify without network calls or a shared secret
// file. The token is derived by computing HMAC-SHA256 of the string
// "cluster:<name>" using the existing ego.server.token.key as the signing key.
//
// Because all nodes in a cluster share the same configuration (and therefore
// the same ego.server.token.key), they will all produce the identical token.
// The "cluster-" prefix ensures the value cannot be confused with a real user
// session token, which would start with a UUID.
//
// The token rotates automatically whenever ego.server.token.key rotates, which
// already requires a server restart — so no additional key-rotation logic is needed.
func clusterToken() string {
	key := settings.Get(defs.ServerTokenKeySetting)
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte("cluster:" + ClusterName))

	return "cluster-" + hex.EncodeToString(mac.Sum(nil))
}

// ValidateClusterToken checks whether the incoming HTTP request carries the
// correct cluster bearer token. It extracts the "Authorization: Bearer <token>"
// header and compares it to the locally computed token using a constant-time
// comparison to resist timing attacks.
//
// Returns true if the token matches; false otherwise. The handler is responsible
// for returning an appropriate HTTP error (e.g. 403 Forbidden) when this
// function returns false.
func ValidateClusterToken(r *http.Request) bool {
	if ClusterName == "" {
		// Not running in cluster mode; reject all cluster control requests.
		return false
	}

	authHeader := r.Header.Get("Authorization")

	if len(authHeader) <= len(prefix) {
		return false
	}

	provided := authHeader[len(prefix):]
	expected := clusterToken()

	// Use hmac.Equal for constant-time comparison so that an attacker cannot
	// learn anything from how long the comparison takes.
	return hmac.Equal([]byte(provided), []byte(expected))
}

// ClusterAuthHeader returns the value to place in the Authorization header when
// this node sends a cluster control request to a peer. Peers validate this with
// ValidateClusterToken on their side.
func ClusterAuthHeader() string {
	return "Bearer " + clusterToken()
}
