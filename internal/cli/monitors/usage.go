package monitors

import (
	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/shared"
)

func registerUsage(parent *cobra.Command) {
	shared.RegisterUsage(parent, "monitors", usageText)
}

const usageText = `MONITORS — Datadog monitor triage reference

COMMANDS
  list     List monitors with optional filters
  get      Get full details for a specific monitor
  search   Search monitors by query text
  create   Create a monitor
  update   Change fields on an existing monitor
  delete   Delete a monitor (requires --yes)
  mute     Silence a monitor temporarily
  unmute   Re-enable a muted monitor

EXAMPLES

  # Find all alerting monitors
  agent-dd monitors list --status alert

  # Find monitors for a specific service
  agent-dd monitors list --tag "service:web-api"

  # Search by name
  agent-dd monitors search --query "CPU" --status alert

  # Get full monitor details
  agent-dd monitors get 12345

  # Mute during investigation (1 hour)
  agent-dd monitors mute 12345 --reason "investigating spike" --end now+1h

  # Unmute after resolution
  agent-dd monitors unmute 12345

  # Validate a new monitor without creating it
  agent-dd monitors create --type "metric alert" \
    --query 'avg(last_5m):avg:system.cpu.user{service:web} > 90' \
    --name "CPU high on web" --dry-run

  # Create it, with thresholds and a notification target
  agent-dd monitors create --type "metric alert" \
    --query 'avg(last_5m):avg:system.cpu.user{service:web} > 90' \
    --name "CPU high on web" --message "@slack-oncall" \
    --tag "service:web" --priority 2 \
    --threshold-critical 90 --threshold-warning 80

  # Raise one threshold — everything else is left alone
  agent-dd monitors update 12345 --threshold-critical 95

  # Options with no dedicated flag: pass the whole definition
  agent-dd monitors create --body @monitor.json
  echo '{"type":"log alert","query":"...","name":"..."}' | agent-dd monitors create --body @-

  # Delete (--yes required; --force if an SLO references it)
  agent-dd monitors delete 12345 --yes

QUERY SYNTAX (--query)
  A monitor query is NOT a metric query — it adds an evaluation window and a
  threshold comparison, and the grammar depends on --type.

  metric alert    time_aggr(window):space_aggr:metric{tags} [by {key}] op N
                  avg(last_5m):avg:system.cpu.user{service:web} > 90
                  sum(last_10m):sum:http.errors{env:prod} > 50
                  avg(last_5m):avg:system.mem.used{*} by {host} > 90
                  windows: last_1m/5m/10m/15m/30m/1h/4h/1d
                  aggregations: avg, sum, min, max   operators: > >= < <= ==
                  "by {key}" makes it a multi-alert — one notification per group

  log alert       logs("<log query>").index("<i>").rollup("<m>")[.by("<facet>")].last("<w>") op N
                  logs("service:web AND status:error").index("*").rollup("count").last("5m") > 10

  service check   "<check>".over(tags).last(count)[.by(group)].count_by_status()
                  "datadog.agent.up".over("service:web").last(3).count_by_status()
                  over(...) is required; last(count) >= your largest threshold

  The threshold in the query should match --threshold-critical. Always
  --dry-run first — Datadog parses it with the engine that would run it.

WRITING MONITORS
  --dry-run     Validates against Datadog's own validate endpoint and writes
                nothing. The query is parsed by the engine that would run it,
                so a malformed one fails here rather than being created broken.
                Needs only monitors_read; creating and updating need
                monitors_write.

  update        Changes only the fields you pass. The monitor is read, your
                changes are layered on, and the whole definition is written
                back — so fields this CLI does not model (restricted_roles,
                composite sub-monitors, the long tail of options) survive
                untouched. Options merge key-by-key, so raising a critical
                threshold will not clear the warning one. Tags and other lists
                replace wholesale, since merging them would make removal
                impossible.

                The response carries a "changes" map of exactly what moved,
                keyed by dotted path (options.thresholds.critical). Report that
                back — it is the evidence nothing else changed.

  delete        Requires --yes. Unlike create and update it leaves nothing
                behind to read or correct. Datadog refuses to delete a monitor
                referenced by an SLO or composite monitor; --force overrides,
                but read the refusal first — it usually means the monitor is
                load-bearing.

  --body        Full monitor JSON: inline, @file, or @- for stdin. Individual
                flags override matching fields in --body, so you can start from
                a template and adjust one value.

MONITOR STATUSES
  Datadog returns these in title case with spaces; --status accepts either
  spelling and is case-insensitive, so "no_data", "No Data" and "no data" are
  the same filter.

  Datadog value   --status accepts     Meaning
  OK              ok                   Monitor is healthy
  Alert           alert                Monitor is in alert state
  Warn            warn                 Monitor is in warning state
  No Data         no_data              Monitor has received no data
  Ignored         ignored              Monitor is ignored
  Skipped         skipped              Evaluation was skipped
  Unknown         unknown              Status could not be determined

COMPACT vs FULL OUTPUT
  Default output shows: id, name, status, type
  Use --full flag to see query, message, tags, options, timestamps

TIPS
  - Start with "list --status alert" to see what's firing
  - Use "get <id>" to understand the monitor's query and thresholds
  - Mute with --reason and --end so others know why and when it expires
  - Monitor IDs are integers (not strings)
  - Before creating anything, search for an existing monitor covering the same
    signal — updating one beats adding a near-duplicate, which is how alert
    fatigue starts
  - Always --dry-run a new query before creating it
`
