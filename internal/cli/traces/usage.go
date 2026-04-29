package traces

import (
	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/shared"
)

func registerUsage(parent *cobra.Command) {
	shared.RegisterUsage(parent, "traces", usageText)
}

const usageText = `TRACES — Datadog APM trace search reference

COMMANDS
  search     Search for traces/spans
  services   List APM services

EXAMPLES

  # Search traces for a service
  agent-dd traces search --service my-api --from now-30m

  # Search for error traces
  agent-dd traces search --query "status:error" --service web-api

  # List all APM services
  agent-dd traces services

  # List services in a specific environment
  agent-dd traces services --env production

  # Filter services by name
  agent-dd traces services --search checkout

  # Search for slow traces (>1s)
  agent-dd traces search --query "@duration:>1000000000" --service my-api

OUTPUT
  Compact: service, name, resource, duration (ns), status, error flag
  Duration is in nanoseconds

TIPS
  - Use --service to scope searches to a specific service
  - Combine --query with --service for targeted searches
  - Duration values are in nanoseconds (1s = 1000000000ns)
`
