package cli

import (
	libcli "github.com/shhac/lib-agent-cli/cli"
	agentmcp "github.com/shhac/lib-agent-mcp"
	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/apicmd"
	"github.com/shhac/agent-dd/internal/cli/events"
	"github.com/shhac/agent-dd/internal/cli/hosts"
	"github.com/shhac/agent-dd/internal/cli/incidents"
	"github.com/shhac/agent-dd/internal/cli/logs"
	"github.com/shhac/agent-dd/internal/cli/metrics"
	"github.com/shhac/agent-dd/internal/cli/monitors"
	"github.com/shhac/agent-dd/internal/cli/org"
	"github.com/shhac/agent-dd/internal/cli/shared"
	"github.com/shhac/agent-dd/internal/cli/slo"
	"github.com/shhac/agent-dd/internal/cli/traces"
	"github.com/shhac/agent-dd/internal/output"
)

func newRootCmd(version string) *cobra.Command {
	g := &shared.GlobalFlags{}

	root := libcli.NewRoot(libcli.Options{
		Use:           "agent-dd",
		Short:         "Datadog triage CLI for AI agents",
		Version:       version,
		Globals:       &g.Globals,
		DefaultFormat: output.FormatNDJSON,
		UnknownHint:   "run 'agent-dd usage' to see the available domains",
	})

	// --org is Datadog's organization selector — a domain flag, kept as-is. The
	// shared --format/--timeout/--debug are bound by NewRoot via Globals.
	root.PersistentFlags().StringVarP(&g.Org, "org", "o", "", "Organization alias (or set DD_ORG, or DD_API_KEY + DD_APP_KEY)")

	allGlobals := func() *shared.GlobalFlags { return g }

	registerUsageCommand(root)
	org.Register(root)
	monitors.Register(root, allGlobals)
	logs.Register(root, allGlobals)
	metrics.Register(root, allGlobals)
	events.Register(root, allGlobals)
	hosts.Register(root, allGlobals)
	traces.Register(root, allGlobals)
	incidents.Register(root, allGlobals)
	slo.Register(root, allGlobals)
	apicmd.Register(root, allGlobals)

	// Expose the whole command tree as an MCP server (added last, so it reflects
	// the complete tree). --color/--expose are output-shaping, irrelevant to a
	// tool call, so hide them from the generated schemas.
	root.AddCommand(agentmcp.Command(root, agentmcp.WithHiddenFlags("color", "expose")))

	return root
}

// Run builds the root command and hands it to libcli.Run, the single error
// sink: any error that bubbles out — cobra's own usage errors (unknown command,
// bad flag, failed Args validation) or a command error returned without being
// pre-rendered — is written once to stderr in the structured {error,fixable_by,
// hint} contract, then the process exits 1. Commands that pre-render their own
// validation errors return nil, so they never reach this sink twice.
func Run(version string) {
	libcli.Run(newRootCmd(version))
}
