package commands

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/ego/internal/cli/cli"
	"github.com/tucats/ego/internal/cli/tables"
	"github.com/tucats/ego/internal/cli/ui"
	"github.com/tucats/ego/internal/defs"
	"github.com/tucats/ego/internal/errors"
	"github.com/tucats/ego/internal/util/fork"
	"github.com/tucats/ego/internal/i18n"
	"github.com/tucats/ego/internal/runtime/rest"
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
		"id":   response.ServerInfo.ID,
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
			m.NodeID,
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

// ClusterStart starts multiple Ego server instances as members of a named
// cluster, one per port listed in --ports. Each node is launched as a
// detached background process (the same as "ego start server") with the
// --cluster flag set so it registers in the shared membership table.
//
// --ports is a comma-separated list of port numbers (e.g. --ports 4040,4041,4042).
// --cluster is required; the command returns an error if it is absent.
//
// Nodes are started sequentially with a one-second pause between them so
// that their auto-generated log filenames have distinct timestamps.
//
// Invoked by:
//
//	Verb:        ego start cluster --cluster <name> --ports <p1,p2,...>
//	Short verb:  ego cluster start --cluster <name> --ports <p1,p2,...>
//	Traditional: ego server cluster start --cluster <name> --ports <p1,p2,...>
func ClusterStart(c *cli.Context) error {
	clusterName, ok := c.String("cluster")
	if !ok || clusterName == "" {
		return errors.ErrRequiredNotFound.Clone().Context("--cluster")
	}

	portList, ok := c.StringList("ports")
	if !ok || len(portList) == 0 {
		return errors.ErrRequiredNotFound.Clone().Context("--ports")
	}

	// StringListType delivers comma-separated tokens as individual list
	// elements, but the user may also quote the whole list as one string.
	// Flatten and parse all tokens into integer port numbers.
	ports := make([]int, 0, len(portList))

	for _, s := range portList {
		for _, token := range strings.Split(s, ",") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}

			port, err := strconv.Atoi(token)
			if err != nil || port < 1 || port > 65535 {
				return errors.ErrInvalidInteger.Clone().Context(token)
			}

			ports = append(ports, port)
		}
	}

	if len(ports) == 0 {
		return errors.ErrRequiredNotFound.Clone().Context("--ports")
	}

	// Resolve the absolute path of the running ego executable once so all
	// child processes use the same image.
	exePath, err := exec.LookPath(os.Args[0])
	if err != nil {
		exePath = os.Args[0]
	}

	exePath, _ = filepath.Abs(exePath)

	verbMode := strings.Contains(strings.ToLower(os.Getenv("EGO_GRAMMAR")), "verb")

	started := 0

	for i, port := range ports {
		// Brief pause between starts so each node receives a unique
		// log-file timestamp (they share the same base filename stem).
		if i > 0 {
			time.Sleep(time.Second)
		}

		args := buildClusterNodeArgs(exePath, c, clusterName, port, verbMode)

		_, args, err = processServerArguments(c, args)
		if err != nil {
			return err
		}

		pid, startErr := fork.Run(args[0], args)
		if startErr != nil {
			ui.Say("msg.cluster.start.error", ui.A{
				"port":  port,
				"error": startErr.Error(),
			})

			continue
		}

		started++

		ui.Say("msg.cluster.start.node", ui.A{
			"cluster": clusterName,
			"port":    port,
			"pid":     pid,
		})
	}

	if started == 0 {
		return errors.ErrNoNodesStarted
	}

	return nil
}

// buildClusterNodeArgs constructs the command-line argument slice for a single
// cluster node child process. The resulting args can be passed directly to
// processServerArguments (which adds --session-uuid, --users, and --log-file)
// and then to fork.Run.
func buildClusterNodeArgs(exe string, c *cli.Context, clusterName string, port int, verbMode bool) []string {
	args := []string{exe}

	if verbMode {
		// ego server --is-detached ...
		args = append(args, "server", "--is-detached")
	} else {
		// ego server run --is-detached ...
		args = append(args, "server", "run", "--is-detached")
	}

	args = append(args, "--cluster", clusterName)
	args = append(args, "--port", strconv.Itoa(port))

	if c.Boolean("not-secure") {
		args = append(args, "--not-secure")
	}

	if v, ok := c.String("cert-dir"); ok {
		args = append(args, "--cert-dir", v)
	}

	if v, ok := c.Integer("keep-logs"); ok {
		args = append(args, "--keep-logs", strconv.Itoa(v))
	}

	if v, ok := c.String("auth-server"); ok {
		args = append(args, "--auth-server", v)
	}

	if v, ok := c.String("users"); ok {
		args = append(args, "--users", v)
	}

	if v, ok := c.String("superuser"); ok {
		args = append(args, "--superuser", v)
	}

	if v, ok := c.Integer("cache-size"); ok {
		args = append(args, "--cache-size", strconv.Itoa(v))
	}

	if v, ok := c.String(defs.TypingOption); ok {
		args = append(args, "--"+defs.TypingOption, v)
	}

	if v, ok := c.String("realm"); ok {
		args = append(args, "--realm", v)
	}

	if v, ok := c.String("sandbox-path"); ok {
		args = append(args, "--sandbox-path", v)
	}

	if c.Boolean("child-services") {
		args = append(args, "--child-services")
	}

	if c.Boolean("no-log") {
		args = append(args, "--no-log")
	}

	return args
}

// truncateID shortens a UUID-format node ID to just the last 8 hex characters so
// that table columns stay narrow without losing uniqueness in practice.
func truncateID(id string) string {
	if len(id) > 8 {
		return "..." + id[len(id)-8:]
	}

	return id
}
