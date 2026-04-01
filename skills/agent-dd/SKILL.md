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

1. **Identify the signal**: What alerted? Check `monitors search --status alert` or `incidents list`
2. **Scope the time window**: Use `--from now-1h` (or broader) to set the investigation window
3. **Gather context**: Pull logs, metrics, and traces for the affected service
4. **Correlate**: Cross-reference signals — do log errors align with metric spikes? Do traces show latency?

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
agent-dd logs facets --query "service:web" --from now-1h
agent-dd metrics list --search "system.cpu"
agent-dd traces services
agent-dd slo history <id> --from now-7d --to now
```

## Detailed Reference

For full command details with examples, run per-domain help:
```bash
agent-dd llm-help                 # top-level overview
agent-dd logs llm-help
agent-dd monitors llm-help
agent-dd metrics llm-help
agent-dd traces llm-help
agent-dd incidents llm-help
agent-dd slo llm-help
```

## Key Concepts

### Log Query Syntax
Datadog log query language: `service:web status:error @http.method:POST`
- Facets: `@field_name:value`
- Tags: `tag:value`
- Status: `status:(error OR warn)`
- Free text: just include the text
- Boolean: `AND`, `OR`, `NOT`, `-field:value`

### Metric Query Syntax
Datadog metric query: `avg:metric.name{tag:value} by {host}`
- Aggregation: `avg`, `sum`, `min`, `max`, `count`
- Filters: `{tag:value,tag2:value2}` (AND-ed)
- Grouping: `by {tag}` to split series

### Monitor Statuses
`ok`, `alert`, `warn`, `no_data`, `unknown`

### Incident Severities
`SEV-1` through `SEV-5` (1 = most severe)

### Time Formats
- Relative: `now-15m`, `now-1h`, `now-1d`, `now-7d`
- Absolute: RFC3339 `2024-01-15T10:00:00Z`
- Unix epoch seconds

## Organization Setup

If the organization isn't configured yet:
```bash
agent-dd org add <alias> --api-key <key> --app-key <key> [--site datadoghq.com]
agent-dd org test
```
Tell the user to get their keys from Datadog → Organization Settings → API Keys / Application Keys.

Standard Datadog env vars (`DD_API_KEY`, `DD_APP_KEY`, `DD_SITE`) are also supported.
