package metrics

import (
	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/shared"
)

func registerUsage(parent *cobra.Command) {
	shared.RegisterUsage(parent, "metrics", usageText)
}

const usageText = `METRICS — Datadog metric querying reference

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
  Compact by default: {metric, scope, tag_set, pointlist: [[timestamp, value]...]}
  Field names match the v1 /query response. pointlist timestamps are in
  milliseconds (Datadog's native format), values are floats. Series with
  no data return an empty pointlist (not null).

DISCOVERING METRIC NAMES
  'metrics list' requires the metrics_read scope and may return 403 for
  read-only API keys. If you cannot list metrics, common APM-emitted
  patterns to try directly:

    trace.<integration>.request.{hits,errors,duration,duration.95p}
    trace.<integration>.server.{hits,errors,duration,duration.95p}
    trace.<integration>.client.{hits,errors,duration,duration.95p}
    trace.<integration>.query.{hits,errors,duration,duration.95p}

  Where <integration> is e.g. http, grpc, mongo, redis, postgres, kafka,
  sqs, fastify, express, gin, rails. Use .as_count() with sum: to convert
  rate metrics to counts:

    sum:trace.grpc.server.hits{env:prod} by {resource_name}.as_count()
`
