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
`
