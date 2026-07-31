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
  ok        Monitor is healthy
  alert     Monitor is in alert state
  warn      Monitor is in warning state
  no_data   Monitor has no data
  unknown   Monitor status is unknown

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
