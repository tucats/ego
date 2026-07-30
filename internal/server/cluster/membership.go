package cluster

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/runtime/rest"
)

// ListMembers returns all rows from the cluster table for the current cluster
// name. Both "active" and "removed" members are returned so that callers can
// distinguish recently departed nodes from ones that were never in this cluster.
//
// An empty slice (not an error) is returned when no rows exist, which happens
// the first time a cluster is started.
func ListMembers(db *sql.DB, name string) ([]defs.ClusterMember, error) {
	var (
		err  error
		rows *sql.Rows
	)

	if name == "" {
		name = ClusterName
	}

	if name != "" {
		rows, err = db.Query(
			`SELECT name, node_id, host, port, scheme, joined_at, last_seen, state
				FROM cluster
				WHERE name = $1
		  		ORDER BY joined_at`,
			name,
		)
	} else {
		rows, err = db.Query(
			`SELECT name, node_id, host, port, scheme, joined_at, last_seen, state
				FROM cluster
		  		ORDER BY joined_at`,
		)
	}

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

// ListActiveMembers returns only the "active" members of the cluster, INCLUDING
// this node itself. This is the list used to report cluster membership via the
// endpoint /services/cluster/{{name}}.
func ListAllActiveMembers(db *sql.DB, name string) ([]defs.ClusterMember, error) {
	all, err := ListMembers(db, name)
	if err != nil {
		return nil, err
	}

	var active []defs.ClusterMember

	for _, m := range all {
		// Let's verify the node is actually up (assuming it's not ourselves)
		if m.NodeID != NodeID && m.State == "active" {
			urlPath := m.Scheme + "://" + m.Host + ":" + strconv.Itoa(m.Port) + "/services/up"
			resp := defs.RemoteStatusResponse{}

			err := rest.Exchange(urlPath, http.MethodGet, nil, &resp, "ego cluster")

			evict := false
			if err != nil {
				evict = true
			}

			// If this is stale data from a previous cluster incarnation, also not valid.
			if m.NodeID != resp.ID {
				evict = true
			}

			if evict {
				result, err := db.Exec(`UPDATE cluster SET state=$1 WHERE node_id = $2`,
					"inactive", m.NodeID)
				if err != nil {
					return nil, err
				}

				if count, _ := result.RowsAffected(); count != 1 {
					return nil, errors.ErrInternalRuntime.Chain(errors.Message("unable to delete " + m.NodeID))
				}

				m.State = "inactive"
			}
		}

		if m.State == "active" {
			active = append(active, m)
		}
	}

	return active, nil
}

// ListActiveMembers returns only the "active" members of the cluster, excluding
// this node itself. This is the list the health checker and cache-invalidation
// broadcaster use to determine which peers to contact.
func ListActiveMembers(db *sql.DB, name string) ([]defs.ClusterMember, error) {
	all, err := ListMembers(db, name)
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
//
// The two databases require different UPSERT syntax:
//   - SQLite:     INSERT OR REPLACE ... VALUES (...)
//   - PostgreSQL: INSERT ... VALUES (...) ON CONFLICT (node_id) DO UPDATE SET ...
func upsertMember(db *sql.DB, m defs.ClusterMember) error {
	var query string

	if dbProvider == defs.PostgresProvider {
		query = `INSERT INTO cluster
				(name, node_id, host, port, scheme, joined_at, last_seen, state)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 ON CONFLICT (node_id) DO UPDATE SET
			 	name      = EXCLUDED.name,
			 	host      = EXCLUDED.host,
			 	port      = EXCLUDED.port,
			 	scheme    = EXCLUDED.scheme,
			 	joined_at = EXCLUDED.joined_at,
			 	last_seen = EXCLUDED.last_seen,
			 	state     = EXCLUDED.state`
	} else {
		query = `INSERT OR REPLACE INTO cluster
				(name, node_id, host, port, scheme, joined_at, last_seen, state)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	}

	_, err := db.Exec(query,
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
		`UPDATE cluster SET state = 'removed', last_seen = $1 WHERE node_id = $2`,
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
		`UPDATE cluster SET last_seen = $1 WHERE node_id = $2`,
		now, nodeID,
	)

	return err
}
