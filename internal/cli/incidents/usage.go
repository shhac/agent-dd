package incidents

import (
	"github.com/spf13/cobra"

	"github.com/shhac/agent-dd/internal/cli/shared"
)

func registerUsage(parent *cobra.Command) {
	shared.RegisterUsage(parent, "incidents", usageText)
}

const usageText = `INCIDENTS — Datadog incident management reference

COMMANDS
  list     List incidents with optional state filter
  get      Get full incident details
  create   Create a new incident
  update   Update incident state or severity

EXAMPLES

  # List active incidents
  agent-dd incidents list --state active

  # Get incident details
  agent-dd incidents get abc-123-def

  # Create an incident (customer-impacted)
  agent-dd incidents create --title "Elevated error rate in web-api" --severity SEV-3 --customer-impacted

  # Mark an incident as stable
  agent-dd incidents update abc-123-def --state stable

  # Resolve an incident
  agent-dd incidents update abc-123-def --state resolved

SEVERITIES
  SEV-1   Critical — total service outage, customer-facing
  SEV-2   High — major degradation, significant customer impact
  SEV-3   Moderate — partial degradation, some customer impact
  SEV-4   Low — minor issue, minimal customer impact
  SEV-5   Informational — no immediate customer impact

STATES
  active     Incident is being investigated
  stable     Incident is contained but not resolved
  resolved   Incident has been resolved

COMMANDER
  --commander-uuid takes a Datadog user UUID (not handle/email). Find the UUID
  in the Datadog UI under Team > Users. Omit the flag to leave unassigned.

TIPS
  - Always check existing incidents before creating a new one
  - Set severity based on customer impact, not internal complexity
  - Update state as investigation progresses
  - Incident IDs are strings (UUIDs), not integers
`
