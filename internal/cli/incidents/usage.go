package incidents

import (
	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/shared"
)

func registerLLMHelp(parent *cobra.Command) {
	shared.RegisterLLMHelp(parent, "Detailed incidents reference for LLMs", llmHelpText)
}

const llmHelpText = `INCIDENTS — Datadog incident management reference

COMMANDS
  list     List incidents with optional status filter
  get      Get full incident details
  create   Create a new incident
  update   Update incident status or severity

EXAMPLES

  # List active incidents
  agent-dd incidents list --status active

  # Get incident details
  agent-dd incidents get abc-123-def

  # Create an incident
  agent-dd incidents create --title "Elevated error rate in web-api" --severity SEV-3

  # Mark an incident as stable
  agent-dd incidents update abc-123-def --status stable

  # Resolve an incident
  agent-dd incidents update abc-123-def --status resolved

SEVERITIES
  SEV-1   Critical — total service outage, customer-facing
  SEV-2   High — major degradation, significant customer impact
  SEV-3   Moderate — partial degradation, some customer impact
  SEV-4   Low — minor issue, minimal customer impact
  SEV-5   Informational — no immediate customer impact

STATUSES
  active     Incident is being investigated
  stable     Incident is contained but not resolved
  resolved   Incident has been resolved

TIPS
  - Always check existing incidents before creating a new one
  - Set severity based on customer impact, not internal complexity
  - Update status as investigation progresses
  - Incident IDs are strings (UUIDs), not integers
`
