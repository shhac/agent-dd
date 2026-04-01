package metrics

import (
	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/shared"
)

func registerLLMHelp(parent *cobra.Command) {
	shared.RegisterLLMHelp(parent, "Detailed metrics reference for LLMs", llmHelpText)
}

const llmHelpText = `METRICS — Datadog metric querying reference

COMMANDS
  query      Query metric timeseries data
  list       Search for metric names
  metadata   Get metric type, unit, and description

EXAMPLES

  # Query CPU usage for a host
  agent-dd metrics query --query "avg:system.cpu.user{host:web-1}" --from now-1h --to now

  # Query with grouping
  agent-dd metrics query --query "avg:http.requests{env:prod} by {service}" --from now-30m

  # Search metric names
  agent-dd metrics list --search "system.cpu"

  # Get metric metadata
  agent-dd metrics metadata system.cpu.user

METRIC QUERY SYNTAX
  avg:metric.name{tag:value}             Basic query with filter
  sum:metric.name{tag:value} by {host}   Aggregation with grouping
  max:metric.name{*}                     All hosts, max aggregation

  Aggregations: avg, sum, min, max, count
  Filters: {tag:value,tag2:value2}  (AND-ed)
  Grouping: by {tag} to split into separate series

OUTPUT
  Compact by default: {metric, tags, points: [[timestamp, value]...]}
  Points are [unix_epoch_seconds, value] pairs
`
