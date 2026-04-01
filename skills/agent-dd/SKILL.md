---
name: agent-dd
description: Triage and investigate Datadog logs, metrics, monitors, traces, incidents, and SLOs
triggers:
  - datadog
  - dd
  - logs
  - metrics
  - monitors
  - incidents
  - traces
  - apm
  - slo
  - observability
  - alert
  - on-call
  - triage
tools:
  allowed:
    - Bash
    - Read
    - Grep
    - Glob
---

# agent-dd — Datadog Triage CLI

Investigate Datadog logs, metrics, monitors, traces, incidents, and SLOs. Designed for triage and debugging workflows, not full Datadog administration.

## When to Use

- User asks about log errors, spikes, or anomalies
- User wants to check monitor/alert status or mute/unmute monitors
- User is investigating an incident and needs metrics, traces, or logs
- User asks about SLO burn rate or error budget
- User wants to correlate events across Datadog signals

## Process

### Investigation workflow

1. **Identify the signal**: What alerted? Check `monitors list --status alert` or `incidents list --status active`
2. **Scope the time window**: Use `--from now-1h` (or broader) to set the investigation window
3. **Find the hotspot**: Use `logs facets` to see which services/hosts/statuses dominate, then drill in
4. **Gather context**: Pull logs, metrics, and traces for the affected service
5. **Correlate**: Cross-reference signals — do log errors align with metric spikes? Do traces show latency?

### Always read before acting

1. **Check monitor state** before muting: `monitors get <id>`
2. **Check incident status** before updating: `incidents get <id>`
3. **Preview logs** before drawing conclusions: `logs search --query "..." --limit 10`

### Error handling

All errors are JSON to stderr with a classification:
- `fixable_by: agent` — bad query syntax, wrong ID. Read the hint and retry.
- `fixable_by: human` — credentials or permissions issue. Tell the user.
- `fixable_by: retry` — transient error. Wait and retry once.

## Quick Reference

```bash
# Explore (safe, read-only)
agent-dd monitors list --status alert
agent-dd monitors get <id>
agent-dd logs search --query "service:web status:error" --from now-1h
agent-dd logs facets --query "status:error" --from now-1h
agent-dd metrics query --query "avg:system.cpu.user{host:web-1}" --from now-1h --to now
agent-dd traces search --service my-api --from now-30m
agent-dd incidents list --status active
agent-dd slo list
agent-dd hosts list --tag "env:production"

# Triage actions
agent-dd monitors mute <id> --reason "investigating" --end now+1h
agent-dd monitors unmute <id>
agent-dd incidents create --title "Elevated error rate" --severity SEV-3
agent-dd incidents update <id> --status stable

# Discovery
agent-dd metrics list --search "system.cpu"
agent-dd traces services
agent-dd slo history <id> --from now-7d --to now
```

## Query Syntax

### Log queries

Used by `logs search`, `logs tail`, `logs facets`, and `traces search`.

```
service:web-api                    # exact tag match
status:error                       # by log status (error, warn, info, debug)
host:web-1                         # by host
source:nginx                       # by log source
@http.method:POST                  # facet match (@ prefix for attributes)
@http.status_code:>500             # numeric comparison (>, >=, <, <=)
@duration:>1000000                 # works in trace search too (nanoseconds)
"connection timeout"               # free text (quoted for exact phrase)
service:web AND status:error       # boolean AND (implicit between terms)
status:(error OR warn)             # boolean OR
NOT service:internal               # boolean NOT
-service:internal                  # exclusion shorthand
service:web* host:prod-*           # wildcards
```

Start broad, then narrow: `logs facets` shows you which services/hosts/statuses have volume, then add filters to `logs search` to drill in.

### Metric queries

Used by `metrics query`.

```
avg:system.cpu.user{host:web-1}                    # basic: aggregation:metric{filter}
sum:http.requests{env:prod} by {service}            # grouping: split by tag
max:system.disk.used{*}                             # all hosts
avg:app.request.duration{service:api,env:prod}      # multiple filters (AND-ed)
```

Aggregations: `avg`, `sum`, `min`, `max`, `count`.

### Trace queries

Traces use the same log query syntax but with APM-specific facets:

```
agent-dd traces search --query "service:web-api @duration:>1000000000" --from now-30m
agent-dd traces search --query "status:error" --service web-api
agent-dd traces search --service web-api    # all traces for a service
```

Duration is in **nanoseconds** (1s = 1,000,000,000ns). Common facets: `service`, `resource_name`, `@duration`, `status`, `@http.status_code`.

## Key Concepts

### Monitor statuses
`ok`, `alert`, `warn`, `no_data`, `unknown`

### Incident severities
`SEV-1` (critical) through `SEV-5` (informational)

### Incident statuses
`active`, `stable`, `resolved`

### Time formats
- Relative: `now-15m`, `now-1h`, `now-1d`, `now-7d`
- Absolute: RFC3339 `2024-01-15T10:00:00Z`
- Unix epoch seconds
- Defaults: `--from now-1h`, `--to now`

### Output
- Compact by default (e.g. monitors: `id, name, status, type`). Use `--full` for complete API response.
- `--format jsonl` for NDJSON (one object per line).

## Deeper Reference

For full command details, examples, and field descriptions beyond what's covered here:

```bash
agent-dd llm-help                 # top-level command overview
agent-dd logs llm-help            # log query examples, sort options, compact vs full
agent-dd monitors llm-help        # monitor statuses, muting best practices
agent-dd metrics llm-help         # metric query syntax, aggregation details
agent-dd traces llm-help          # trace search, duration units
agent-dd incidents llm-help       # severity guide, lifecycle
agent-dd slo llm-help             # error budgets, history interpretation
```

Only run these when you need specifics not covered above — they add ~500 tokens each.

## Organization Setup

If the organization isn't configured yet:
```bash
agent-dd org add <alias> --api-key <key> --app-key <key> [--site datadoghq.com]
agent-dd org test
```
Tell the user to get their keys from Datadog → Organization Settings → API Keys / Application Keys.

Standard Datadog env vars (`DD_API_KEY`, `DD_APP_KEY`, `DD_SITE`) are also supported.
