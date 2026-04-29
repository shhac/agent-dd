# agent-dd

Datadog triage CLI for AI agents. Investigation workflows — monitors, logs, metrics, traces, incidents, SLOs — not full Datadog administration.

- **Token-efficient output** — NDJSON for lists, JSON for single items, YAML available. Compact and null-pruned by default. `--full` for complete API responses
- **Structured error classification** — every error includes `fixable_by: agent|human|retry` so AI agents can self-correct without parsing messages
- **Triage-focused** — only the commands you need during an investigation, not the 200+ Datadog API endpoints
- **Multi-org support** — switch between Datadog organizations with `--org`, credentials stored in macOS Keychain
- **Self-documenting** — `agent-dd usage` and per-domain `agent-dd <domain> usage` for agent-friendly reference

### Why not the Datadog CLI?

The official Datadog CLI is built for humans — interactive prompts, wide table output, full admin surface. `agent-dd` is built for AI agents: structured JSON to stdout, classified errors to stderr, compact defaults that fit in context windows, and a surface area limited to what matters during triage.

## Installation

```bash
brew install shhac/tap/agent-dd
```

### Claude Code / AI agent skill

```bash
npx skills add shhac/agent-dd
```

### Other options

Download binaries from [GitHub Releases](https://github.com/shhac/agent-dd/releases), or build from source:

```bash
go install github.com/shhac/agent-dd/cmd/agent-dd@latest
```

## Quick start

### 1. Add a Datadog organization

```bash
agent-dd org add prod --api-key <DD_API_KEY> --app-key <DD_APP_KEY> --site datadoghq.com
agent-dd org test
```

Or use environment variables directly (no setup needed):

```bash
export DD_API_KEY=<key>
export DD_APP_KEY=<key>
```

### 2. Check what's alerting

```bash
# All firing monitors
agent-dd monitors list --status alert

# Full details for a specific monitor
agent-dd monitors get 12345
```

### 3. Investigate with logs and traces

```bash
# Search logs for a service
agent-dd logs search --query "service:web-api status:error" --from now-15m

# Top error facets
agent-dd logs facets --query "status:error" --from now-1h

# Search traces for slow requests
agent-dd traces search --service web-api --query "@duration:>1000000000" --from now-30m
```

## Command map

```text
agent-dd
├── org           add, update, remove, list, set-default, test
├── monitors      list, get, search, mute, unmute, usage
├── logs          search, tail, facets, usage
├── metrics       query, list, metadata, usage
├── events        list, get
├── hosts         list, get, mute
├── traces        search, services, usage
├── incidents     list, get, create, update, usage
├── slo           list, get, history, usage
├── usage         top-level reference card
└── version
```

## Output

- **stdout** — NDJSON for list/search commands (one object per line), JSON for single-item commands
- **stderr** — errors as JSON with `fixable_by` classification
- **`--format json|yaml|jsonl`** — override the default for any command
- **Compact by default** — e.g. monitors show `id, name, status, type`. Use `--full` for everything
- **Null-pruned** — empty/null fields stripped from output to save tokens

## Error output

All errors are written to stderr as structured JSON:

```json
{"error": "Not found: monitor 99999", "fixable_by": "agent", "hint": "Check the ID — use 'list' to see available items"}
```

| `fixable_by` | Meaning |
|---|---|
| `agent` | Bad request — the agent should fix its parameters and retry |
| `human` | Auth or permissions — the agent should stop and ask the human |
| `retry` | Transient — rate limit or server error, wait and retry |

## Multi-org support

```bash
# Add multiple organizations
agent-dd org add prod --api-key <key> --app-key <key> --site datadoghq.com
agent-dd org add eu --api-key <key> --app-key <key> --site datadoghq.eu

# Query a specific org
agent-dd monitors list --status alert --org eu

# Set a default
agent-dd org set-default prod
```

## Time formats

All `--from` / `--to` flags accept:

- **Relative** — `now-15m`, `now-1h`, `now-7d`, `now+1h`
- **RFC3339** — `2024-01-15T10:00:00Z`
- **Unix epoch** — `1705312800`

Defaults: `--from now-1h`, `--to now`.

## Environment variables

| Variable | Purpose |
|---|---|
| `DD_API_KEY` + `DD_APP_KEY` | Direct credential auth (skips org config) |
| `DD_SITE` | Datadog site domain (e.g. `datadoghq.com`, `datadoghq.eu`) |
| `DD_ORG` | Default organization alias |
| `DD_API_URL` | Override base API URL (e.g. `http://localhost:8321/api` for mock server) |

## Development

```bash
make build          # Build binary
make test           # Run all tests
make vet            # Go vet
make dev ARGS="monitors list --status alert"
make mock           # Start mock Datadog API on :8321
make mock-dev ARGS="monitors list"  # Run CLI against mock server
```

## License

MIT
