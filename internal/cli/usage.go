package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func registerUsageCommand(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:   "usage",
		Short: "LLM-optimized reference card",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(usageText)
		},
	})
}

const usageText = `agent-dd — Datadog triage CLI for AI agents

ORGANIZATION SETUP
  agent-dd org add <alias> --api-key <key> --app-key <key> [--site <site>]
  agent-dd org test
  agent-dd org list
  agent-dd org set-default <alias>

MONITORS (triage starting point)
  agent-dd monitors list [--status alert|warn|ok|no_data] [--tag <tag>]
  agent-dd monitors get <id>
  agent-dd monitors search --query <text> [--status <status>]
  agent-dd monitors mute <id> [--end <time>] [--reason <text>]
  agent-dd monitors unmute <id>

LOGS (investigation)
  agent-dd logs search --query <query> [--from <time>] [--to <time>] [--limit N]
  agent-dd logs tail --query <query>
  agent-dd logs facets --query <query> [--from <time>] [--to <time>]

METRICS
  agent-dd metrics query --query <dd-query> --from <time> --to <time>
  agent-dd metrics list [--search <text>] [--tag <tag>]
  agent-dd metrics metadata <metric-name>

EVENTS
  agent-dd events list [--from <time>] [--to <time>] [--source <src>]
  agent-dd events get <id>

HOSTS
  agent-dd hosts list [--search <text>] [--tag <tag>]
  agent-dd hosts get <hostname>
  agent-dd hosts mute <hostname> [--end <time>] [--reason <text>]

TRACES (APM)
  agent-dd traces search --query <query> [--service <svc>] [--from <time>] [--to <time>]
  agent-dd traces services [--search <text>] [--env <env>]

INCIDENTS
  agent-dd incidents list [--status <status>]
  agent-dd incidents get <id>
  agent-dd incidents create --title <text> --severity <SEV-1..5>
  agent-dd incidents update <id> [--status <status>] [--severity <sev>]

SLOs
  agent-dd slo list [--search <text>] [--tag <tag>]
  agent-dd slo get <id>
  agent-dd slo history <id> --from <time> --to <time>

TIME FORMATS
  Relative: now-15m, now-1h, now-1d, now-7d
  Absolute: 2024-01-15T10:00:00Z (RFC3339)
  Unix epoch seconds

GLOBAL FLAGS
  -o, --org <alias>    Organization alias (or DD_ORG env, or DD_API_KEY + DD_APP_KEY)
  --format json|yaml|jsonl   (default: jsonl for lists, json for single items)
  --timeout <ms>

Per-domain details: agent-dd <domain> usage
`
