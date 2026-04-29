package slo

import (
	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/shared"
)

func registerUsage(parent *cobra.Command) {
	shared.RegisterUsage(parent, "slo", usageText)
}

const usageText = `SLO — Datadog Service Level Objectives reference

COMMANDS
  list      List SLOs with optional filters
  get       Get full SLO details (thresholds, description)
  history   Get SLO history over a time range

EXAMPLES

  # List all SLOs
  agent-dd slo list

  # Search SLOs by name
  agent-dd slo list --search "availability"

  # Get SLO details
  agent-dd slo get abc123def456

  # Check SLO history for the past week
  agent-dd slo history abc123def456 --from now-7d --to now

  # Check SLO history for the past 30 days
  agent-dd slo history abc123def456 --from now-30d --to now

KEY FIELDS
  status                   Current SLI value (0-100%)
  error_budget_remaining   Remaining error budget (0-100%)
  thresholds               Target SLI per timeframe (7d, 30d, 90d)

SLO TYPES
  metric    Based on metric queries
  monitor   Based on monitor uptime

TIPS
  - error_budget_remaining < 0 means the SLO is breached
  - Check history to see burn rate trends
  - SLO IDs are strings, not integers
`
