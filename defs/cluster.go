package defs

// ClusterMember describes a single Ego server node that is participating in a
// named cluster. A row for each member is stored in the "cluster" system table
// so every node in the cluster can discover its peers.
type ClusterMember struct {
	// Name is the cluster name supplied via the --cluster flag at startup.
	Name string `json:"name"`

	// NodeID is the server instance UUID that uniquely identifies this process.
	// It matches defs.InstanceID and is stable for the lifetime of one server run.
	NodeID string `json:"node_id"`

	// Host is the hostname or IP address on which this node is listening.
	Host string `json:"host"`

	// Port is the TCP port on which this node is listening.
	Port int `json:"port"`

	// Scheme is either "http" or "https" and describes how peers should connect.
	Scheme string `json:"scheme"`

	// JoinedAt is the RFC3339 timestamp when this node joined the cluster.
	JoinedAt string `json:"joined_at"`

	// LastSeen is the RFC3339 timestamp of the most recent successful health
	// check ping received from (or sent to) this node.
	LastSeen string `json:"last_seen"`

	// State is either "active" (the node is healthy and participating) or
	// "removed" (the node has left or been evicted from the cluster).
	State string `json:"state"`
}

// ClusterStatusResponse is the JSON body returned by GET /services/cluster.
// It lists all known members of the cluster, including recently removed ones,
// together with identifying information about the node that served the request.
type ClusterStatusResponse struct {
	// ClusterName is the name of the cluster as passed to --cluster at startup.
	ClusterName string `json:"cluster"`

	// Members is the full list of cluster members from the system database.
	Members []ClusterMember `json:"members"`

	ServerInfo `json:"server"`

	Status int `json:"status"`

	Message string `json:"msg"`
}

// ClusterFlushRequest is the JSON body sent by POST /services/cluster/flush/{cache-id}.
// It tells the receiving node which in-memory cache to discard so that the next
// access re-reads fresh data from the shared system database.
type ClusterFlushRequest struct {
	// CacheID is the integer cache class constant (e.g. caches.UserCache) that
	// should be purged on the receiving node.
	CacheID int `json:"cache_id"`

	// SenderID is the NodeID of the server that detected the change and is
	// requesting the flush. Used for logging only.
	SenderID string `json:"sender_id"`
}

// ClusterCacheNames maps integer cache class constants to human-readable names
// for use in log messages. This avoids importing the caches package from defs.
var ClusterCacheNames = map[int]string{
	0: "DSNCache",
	1: "AuthCache",
	2: "UserCache",
	3: "TokenCache",
	4: "BlacklistCache",
	5: "SchemaCache",
	6: "SymbolTableCache",
	7: "DebugSessionCache",
	8: "WebAuthnChallengeCache",
}
