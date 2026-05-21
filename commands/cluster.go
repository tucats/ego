package commands

import (
	"fmt"
	"net/http"

	"github.com/tucats/ego/app-cli/cli"
	"github.com/tucats/ego/app-cli/tables"
	"github.com/tucats/ego/app-cli/ui"
	"github.com/tucats/ego/defs"
	"github.com/tucats/ego/errors"
	"github.com/tucats/ego/i18n"
	"github.com/tucats/ego/runtime/rest"
)

// ClusterShow is the CLI handler for showing the current cluster membership state.
// It sends GET /services/cluster to the configured server and displays the
// list of cluster members — their node IDs, host addresses, ports, states, and
// join/last-seen timestamps — as a formatted table.
//
// If the target server is not running in cluster mode it returns a 404, which
// this command translates into a user-friendly message.
//
// Invoked by:
//
//	Traditional: ego server cluster show
//	Verb:        ego cluster server show
func ClusterShow(c *cli.Context) error {
	response := defs.ClusterStatusResponse{}

	if err := rest.Exchange(defs.ServicesClusterPath, http.MethodGet, nil, &response, defs.AdminAgent); err != nil {
		return err
	}

	if ui.OutputFormat != ui.TextFormat {
		return c.Output(response)
	}

	fmt.Printf("%s\n", i18n.M("server.cluster", map[string]any{
		"name": response.ClusterName,
		"id":   response.ServerID,
	}))

	if len(response.Members) == 0 {
		fmt.Printf("  %s\n\n", i18n.M("server.cluster.no.members"))

		return nil
	}

	t, _ := tables.New([]string{
		i18n.L("cluster.node.id"),
		i18n.L("cluster.host"),
		i18n.L("cluster.port"),
		i18n.L("cluster.state"),
		i18n.L("cluster.joined"),
		i18n.L("cluster.seen"),
	})

	_ = t.SetAlignment(2, tables.AlignmentRight)

	for _, m := range response.Members {
		_ = t.AddRowItems(
			truncateID(m.NodeID),
			m.Host,
			m.Port,
			m.State,
			m.JoinedAt,
			m.LastSeen,
		)
	}

	_ = t.SetIndent(2)
	t.SetPagination(0, 0)

	fmt.Println()
	t.Print(ui.TextFormat)
	fmt.Println()

	return nil
}

// ClusterStopNode is the CLI handler for stopping one or all nodes in a cluster.
// With --node <id> it stops a single node; with --all it stops every active node.
//
// To stop a node, the command first retrieves the cluster membership list from
// the currently-configured server, then sends POST /services/cluster/shutdown
// directly to each target node using admin credentials.
//
// Invoked by:
//
//	Traditional: ego server cluster stop [--node <id>|--all]
//	Verb:        ego cluster server stop [--node <id>|--all]
func ClusterStopNode(c *cli.Context) error {
	nodeID, hasNode := c.String("node")
	allNodes := c.Boolean("all")

	if !hasNode && !allNodes {
		return errors.ErrRequiredNotFound.Clone().Context("--node or --all")
	}

	// Retrieve cluster membership to discover each node's URL.
	membership := defs.ClusterStatusResponse{}
	if err := rest.Exchange(defs.ServicesClusterPath, http.MethodGet, nil, &membership, defs.AdminAgent); err != nil {
		return err
	}

	stopped := 0

	for _, m := range membership.Members {
		if m.State != "active" {
			continue
		}

		if !allNodes && m.NodeID != nodeID {
			continue
		}

		// Build the full URL for this node so rest.Exchange talks to it directly
		// (rest.Exchange uses the URL as-is when it starts with a scheme).
		targetURL := fmt.Sprintf("%s://%s:%d%s", m.Scheme, m.Host, m.Port, defs.ServicesClusterShutdownPath)

		var result map[string]any
		if err := rest.Exchange(targetURL, http.MethodPost, nil, &result, defs.AdminAgent); err != nil {
			ui.Log(ui.ServerLogger, "cluster.shutdown.error", ui.A{
				"id":    m.NodeID,
				"error": err.Error(),
			})

			if !allNodes {
				return err
			}
		} else {
			stopped++

			if ui.OutputFormat == ui.TextFormat {
				fmt.Printf("%s\n", i18n.M("server.cluster.stopped", map[string]any{
					"id": truncateID(m.NodeID),
				}))
			}
		}
	}

	if stopped == 0 {
		if hasNode {
			return errors.ErrNotFound.Clone().Context(nodeID)
		}

		fmt.Println(i18n.M("server.cluster.no.active"))
	}

	return nil
}

// ClusterRemoveNode is the CLI handler for forcibly evicting a non-responsive
// node from the cluster membership table. Unlike ClusterStopNode, this does not
// contact the target node at all; it simply marks the node "removed" in the
// cluster database on the configured server.
//
// Use this when a node has crashed and the health checker has not yet evicted it,
// or when you need to remove a node immediately without waiting for the 90-second
// timeout.
//
// Invoked by:
//
//	Traditional: ego server cluster remove --node <id>
//	Verb:        ego cluster server remove --node <id>
func ClusterRemoveNode(c *cli.Context) error {
	nodeID, found := c.String("node")
	if !found || nodeID == "" {
		return errors.ErrRequiredNotFound.Clone().Context("--node")
	}

	url := rest.URLBuilder(defs.ServicesClusterRemovePath).Parameter("node_id", nodeID).String()

	var result map[string]any

	if err := rest.Exchange(url, http.MethodPost, nil, &result, defs.AdminAgent); err != nil {
		return err
	}

	if ui.OutputFormat == ui.TextFormat {
		fmt.Printf("%s\n", i18n.M("server.cluster.removed", map[string]any{
			"id": truncateID(nodeID),
		}))
	} else {
		return c.Output(result)
	}

	return nil
}

// truncateID shortens a UUID-format node ID to just the last 8 hex characters so
// that table columns stay narrow without losing uniqueness in practice.
func truncateID(id string) string {
	if len(id) > 8 {
		return "..." + id[len(id)-8:]
	}

	return id
}
