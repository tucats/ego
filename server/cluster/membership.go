package cluster

import (
	"database/sql"
	"time"

	"github.com/tucats/ego/defs"
)

// ListMembers returns all rows from the cluster table for the current cluster
// name. Both "active" and "removed" members are returned so that callers can
// distinguish recently departed nodes from ones that were never in this cluster.
//
// An empty slice (not an error) is returned when no rows exist, which happens
// the first time a cluster is started.
func ListMembers(db *sql.DB) ([]defs.ClusterMember, error) {
	rows, err := db.Query(
		`SELECT name, node_id, host, port, scheme, joined_at, last_seen, state
		   FROM cluster
		  WHERE name = ?
		  ORDER BY joined_at`,
		ClusterName,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var members []defs.ClusterMember

	for rows.Next() {
		var m defs.ClusterMember

		if err := rows.Scan(
			&m.Name, &m.NodeID, &m.Host, &m.Port,
			&m.Scheme, &m.JoinedAt, &m.LastSeen, &m.State,
		); err != nil {
			return nil, err
		}

		members = append(members, m)
	}

	return members, rows.Err()
}

// ListActiveMembers returns only the "active" members of the cluster, excluding
// this node itself. This is the list the health checker and cache-invalidation
// broadcaster use to determine which peers to contact.
func ListActiveMembers(db *sql.DB) ([]defs.ClusterMember, error) {
	all, err := ListMembers(db)
	if err != nil {
		return nil, err
	}

	var active []defs.ClusterMember

	for _, m := range all {
		if m.State == "active" && m.NodeID != NodeID {
			active = append(active, m)
		}
	}

	return active, nil
}

// upsertMember writes a member row to the cluster table, inserting it if the
// node_id does not exist or replacing the entire row if it does. This is used
// both when a node first joins and when the health checker updates last_seen.
func upsertMember(db *sql.DB, m defs.ClusterMember) error {
	_, err := db.Exec(
		`INSERT OR REPLACE INTO cluster
			(name, node_id, host, port, scheme, joined_at, last_seen, state)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.Name, m.NodeID, m.Host, m.Port,
		m.Scheme, m.JoinedAt, m.LastSeen, m.State,
	)

	return err
}

// RemoveMember sets a cluster member's state to "removed" and stamps its
// last_seen to the current time. It does not delete the row so that the
// departure event remains visible in cluster status output.
func RemoveMember(db *sql.DB, nodeID string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := db.Exec(
		`UPDATE cluster SET state = 'removed', last_seen = ? WHERE node_id = ?`,
		now, nodeID,
	)

	return err
}

// UpdateLastSeen refreshes the last_seen timestamp for this node. It is called
// by the health checker after a successful round of pings to confirm that this
// node is still running, so that peers do not evict it.
func UpdateLastSeen(db *sql.DB, nodeID string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := db.Exec(
		`UPDATE cluster SET last_seen = ? WHERE node_id = ?`,
		now, nodeID,
	)

	return err
}
