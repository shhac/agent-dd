package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

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
)

var (
	flagOrg     string
	flagFormat  string
	flagTimeout int
)

func allGlobals() *shared.GlobalFlags {
	return &shared.GlobalFlags{
		Org:     flagOrg,
		Format:  flagFormat,
		Timeout: flagTimeout,
	}
}

func newRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "agent-dd",
		Short:         "Datadog triage CLI for AI agents",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVarP(&flagOrg, "org", "o", "", "Organization alias (or set DD_ORG, or DD_API_KEY + DD_APP_KEY)")
	root.PersistentFlags().StringVar(&flagFormat, "format", "", "Output format: json, yaml, jsonl")
	root.PersistentFlags().IntVar(&flagTimeout, "timeout", 0, "Request timeout in milliseconds")

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

	return root
}

func Execute(version string) error {
	err := newRootCmd(version).Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return err
}
