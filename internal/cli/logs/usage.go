package logs

import (
	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/shared"
)

func registerUsage(parent *cobra.Command) {
	shared.RegisterUsage(parent, "logs", usageText)
}

const usageText = `LOGS — Datadog log search and analysis reference

COMMANDS
  search   Search logs with query syntax
  tail     Poll recent logs (streams with --follow)
  facets   Get top facet values for a query

EXAMPLES

  # Search for errors in a service
  agent-dd logs search --query "service:web-api status:error" --from now-1h

  # Search with free text
  agent-dd logs search --query "timeout connection refused" --limit 20

  # Tail recent logs (last 5 minutes)
  agent-dd logs tail --query "service:web-api" --service web-api

  # Stream logs continuously (poll every 5s)
  agent-dd logs tail --query "service:web-api" --follow

  # Stream with custom interval
  agent-dd logs tail --query "service:web-api" --follow --interval 10

  # Get facet breakdown
  agent-dd logs facets --query "status:error" --from now-1h

QUERY SYNTAX (Datadog log query language)
  service:web-api              Filter by service
  status:error                 Filter by status (error, warn, info, debug)
  host:web-1                   Filter by host
  source:nginx                 Filter by source
  @http.method:POST            Filter by facet
  @http.status_code:>500       Numeric facet comparison
  "connection timeout"         Free text search (quoted)
  status:(error OR warn)       Boolean OR
  NOT service:internal         Boolean NOT
  -service:internal            Exclusion shorthand

COMPACT vs FULL OUTPUT
  Default: timestamp, service, status, message (token-efficient)
  --full: includes host, tags, all attributes

SORT
  --sort asc    Oldest first
  --sort desc   Newest first (default)

STREAMING (tail)
  --follow, -f     Stream continuously instead of one-shot
  --interval N     Poll interval in seconds (default: 5, requires --follow)
  --source         Append source:X to the query
  --service        Append service:X to the query
`
