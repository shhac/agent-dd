---
name: agent-dd
description: Triage and investigate Datadog monitors, logs, metrics, traces, incidents, and SLOs, and create or adjust monitors to cover what you find. Use when the user asks about alerts, log errors, metric spikes, trace latency, incident management, SLO burn rate, error budgets, or on-call triage in Datadog — or asks to create, edit, retune, or delete a Datadog monitor, change alert thresholds, or add alerting for something that just broke.
allowed-tools: Bash(agent-dd *) Read Grep Glob
---

# agent-dd — Datadog Triage CLI

Investigate Datadog monitors, logs, metrics, traces, incidents, and SLOs, and act on what you find by creating or adjusting monitors. Triage, debugging and the hardening that follows — not full Datadog administration (no dashboards, users, roles, pipelines, or synthetics).

## When to Use

- Checking monitor/alert status, muting/unmuting monitors
- Creating a monitor to catch a problem you just diagnosed, or adjusting one that misfired
- Searching logs for errors, spikes, or anomalies
- Querying metrics or investigating metric spikes
- Searching traces for latency or errors
- Managing incidents (list, create, update)
- Checking SLO burn rate or error budget

## Process

### Investigation workflow

1. **Identify the signal**: `monitors list --status alert` or `incidents list --state active`
2. **Scope the time window**: `--from now-1h` (or broader)
3. **Find the hotspot**: `logs facets` to see which services/hosts/statuses dominate
4. **Gather context**: Pull logs, metrics, and traces for the affected service
5. **Correlate**: Do log errors align with metric spikes? Do traces show latency?

### Harden after diagnosis

Once you know what broke, the next questions are "how do we stop it happening
again" and "how do we get visibility on it". Both mean writing a monitor.

1. **Check what already exists** — `monitors search --query "<service>"`. Often
   the right answer is fixing a monitor that misfired, not adding a new one.
2. **Dry-run first** — `--dry-run` sends the definition to Datadog's validate
   endpoint. The query is parsed by the same engine that would run it, so a
   malformed one fails here rather than being created broken.
3. **Then write it** — `monitors create`, or `monitors update <id>` to adjust an
   existing monitor's thresholds.
4. **Report the diff** — `update` returns a before/after of exactly what moved.
   Hand that to the human: it is the evidence that nothing else changed.

Prefer `update` on an existing monitor over creating a near-duplicate. Two
monitors covering the same signal is how alert fatigue starts.

### Always read before acting

- Check monitor state before muting: `monitors get <id>`
- Check incident status before updating: `incidents get <id>`
- Preview logs before drawing conclusions: `logs search --query "..." --limit 10`

### Error handling

Errors are JSON to stderr with a classification:
- `fixable_by: agent` — bad query syntax or wrong ID. Read the hint and retry.
- `fixable_by: human` — credentials or permissions. Tell the user.
- `fixable_by: retry` — transient error. Wait and retry once.

## Quick Reference

```bash
# Explore (read-only)
agent-dd monitors list --status alert
agent-dd monitors get <id>...
agent-dd logs search --query "service:web status:error" --from now-1h
agent-dd logs facets --query "status:error" --from now-1h
agent-dd metrics query --query "avg:system.cpu.user{host:web-1}" --from now-1h --to now
agent-dd traces search --service my-api --from now-30m
agent-dd incidents list --state active
agent-dd incidents get <id>...
agent-dd slo list
agent-dd slo get <id>...
agent-dd hosts list --tag "env:production"
agent-dd hosts get <hostname>...
agent-dd events get <id>...

# Triage actions
agent-dd monitors mute <id> --reason "investigating" --end now+1h
agent-dd monitors unmute <id>
agent-dd incidents create --title "Elevated error rate" --severity SEV-3
agent-dd incidents update <id> --state stable

# Harden (write) — always --dry-run first
agent-dd monitors create --type "metric alert" \
  --query 'avg(last_5m):avg:system.cpu.user{service:web} > 90' \
  --name "CPU high on web" --message "@slack-oncall" \
  --tag "service:web" --priority 2 \
  --threshold-critical 90 --threshold-warning 80 --dry-run
agent-dd monitors update <id> --threshold-critical 95   # merges; siblings survive
agent-dd monitors update <id> --renotify-interval 30 --dry-run
agent-dd monitors delete <id> --yes [--force]

# Options this CLI has no flag for: pass the whole definition
agent-dd monitors create --body @monitor.json
echo '{"type":"log alert","query":"...","name":"..."}' | agent-dd monitors create --body @-

# Discovery
agent-dd metrics list --search "system.cpu"
agent-dd traces services [--env production] [--search checkout]
agent-dd slo history <id> --from now-7d --to now
```

## Query Syntax

Log queries: `service:web status:error @http.status_code:>500 "timeout"`
Metric queries: `avg:system.cpu.user{host:web-1} by {service}`
Trace queries: same as log syntax, with `@duration:>1000000000` (nanoseconds)
Monitor queries: `avg(last_5m):avg:system.cpu.user{service:web} > 90`

**A monitor query is not a metric query.** It adds an evaluation window
(`avg(last_5m):`) and a threshold comparison (`> 90`), and the grammar differs
per `--type` — `log alert` and `service check` look nothing like the above.
Read [references/query-syntax.md](references/query-syntax.md#monitor-queries)
before writing one, and `--dry-run` it.

For full operator reference (wildcards, booleans, numeric comparisons, facets): see [references/query-syntax.md](references/query-syntax.md)

## Key Concepts

- **Time formats**: relative (`now-15m`, `now-1h`, `now-7d`), RFC3339, or unix epoch. Defaults: `--from now-1h`, `--to now`
- **Output**: NDJSON by default for all commands (list, search, and single-item get). `--full` for complete API response. `--format json|yaml|jsonl` to override. `get <id>...` accepts 1..N ids — see Get contract below.
- **Monitor statuses**: Datadog returns `OK`, `Alert`, `Warn`, `No Data`, `Ignored`, `Skipped`, `Unknown`. `--status` is case-insensitive and treats spaces and underscores alike, so `alert`, `Alert`, `no_data` and `No Data` all work — but the value in output is Datadog's spelling
- **The state key is always `status`** on a monitor object, in every command. At the top level of `create`/`update`/`delete`/`mute` output, `status` instead reports the command's outcome (`"updated"`, `"deleted"`) — nesting tells them apart
- **Incident severities**: `SEV-1` (critical) through `SEV-5` (informational)
- **Incident statuses**: `active`, `stable`, `resolved`

## Get Contract

`get <id>...` takes one or more ids and returns one result per id, in input order. Default output is NDJSON: one line per id — the record, or `{"@unresolved":{"id","reason","fixable_by","hint"?}}` for an id that couldn't be resolved (e.g. not found / bad id). `--format json|yaml` collapses to one `{"data":[…], "@unresolved":[…]}` envelope. A single `get <id>` is just the one-element case (NDJSON one line by default; pass `--format json` for the object). Item-level misses stay on stdout and exit 0; only a command-level failure (auth, network) goes to stderr with exit 1 and empty stdout.

## Deeper Reference

Per-domain details with examples and field descriptions (only load when you need specifics not covered above):

```bash
agent-dd usage                    # top-level command overview
agent-dd logs usage               # log query examples, sort options, compact vs full
agent-dd monitors usage           # monitor statuses, muting best practices
agent-dd metrics usage            # metric query syntax, aggregation details
agent-dd traces usage             # trace search, duration units
agent-dd incidents usage          # severity guide, lifecycle
agent-dd slo usage                # error budgets, history interpretation
```

## Organization Setup — never paste secrets

The api-key and app-key are secrets. If a user pastes either into chat, **do not**
put it into `--api-key` / `--app-key`: the value would land in your context window,
shell history, `ps`/`/proc`, and any downstream transcript. Keep the secret off argv.

```bash
# Preferred (interactive): the user types both keys into a native OS dialog. The
# secret goes straight into the OS and is never seen by the agent driving the CLI.
agent-dd org add <alias> --form [--site datadoghq.com]

# Non-interactive: pipe the two keys on stdin, one per line — api-key first,
# app-key second. Nothing sensitive touches the command line.
printf '%s\n%s' "$API_KEY" "$APP_KEY" | agent-dd org add <alias> [--site datadoghq.com]

agent-dd org test
```

Keys are in Datadog → Organization Settings → API Keys / Application Keys.

`--form` opens a native dialog (macOS osascript, Linux zenity/kdialog, Windows Win32);
it requires a graphical desktop session and fails cleanly (`fixable_by=human`) on
headless/SSH hosts — surface the hint, do not retry. The piped-stdin form is the
non-interactive fallback: the two-line contract is all-or-nothing (stdin is read only
when **neither** key is passed as a flag), line 1 → api-key, line 2 → app-key. The agent
may set `--site` on the user's behalf, but secret values must always come through
`--form` or piped stdin, never as flag arguments.

`org update <alias>` accepts the same `--form` and piped-stdin entry, and merges
partially: an omitted key preserves the stored value.

Environment variables also work for direct auth: `DD_API_KEY`, `DD_APP_KEY`, `DD_SITE`.
