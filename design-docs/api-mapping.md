# API Mapping

How CLI commands map to Datadog API endpoints.

## Monitors (v1) + Downtimes (v2)

| Command | Method | Endpoint |
|---|---|---|
| `monitors list` | GET | `/v1/monitor` |
| `monitors get <id>` | GET | `/v1/monitor/{id}` |
| `monitors search --query <q>` | GET | `/v1/monitor/search` |
| `monitors mute <id>` | POST | `/v2/downtime` |
| `monitors unmute <id>` | GET + DELETE | `/v2/downtime?filter[monitor_id]=...&filter[status]=active` then DELETE `/v2/downtime/{id}` |

Notes: Status filtering in `list` and `search` is done client-side. Muting creates a v2 downtime scoped to the monitor. Unmuting finds and cancels only **active** downtimes for that monitor — scheduled future downtimes are left intact.

## Logs (v2)

| Command | Method | Endpoint |
|---|---|---|
| `logs search --query <q>` | POST | `/v2/logs/events/search` |
| `logs tail --query <q>` | POST | `/v2/logs/events/search` |
| `logs facets --query <q>` | POST | `/v2/logs/analytics/aggregate` |

Notes: `tail` fetches the last 5 minutes by default. With `--follow`, it polls continuously at `--interval` seconds (default 5s), advancing the time window each tick. `facets` hardcodes `count` aggregation with `limit: 10, order: desc` per group-by facet.

## Metrics (v1/v2)

| Command | Method | Endpoint |
|---|---|---|
| `metrics query --query <q>` | GET | `/v1/query?query=...&from=...&to=...` |
| `metrics list` | GET | `/v2/metrics?filter[metric]=...&filter[tags]=...` |
| `metrics metadata <name>` | GET | `/v1/metrics/{name}` |

Notes: `list` uses the v2 metrics endpoint for both search and tag filtering via `filter[metric]` and `filter[tags]` parameters.

## Events (v1)

| Command | Method | Endpoint |
|---|---|---|
| `events list` | GET | `/v1/events?start=...&end=...` |
| `events get <id>` | GET | `/v1/events/{id}` |

Notes: `get` unwraps the `{"event": {...}}` envelope.

## Hosts (v1)

| Command | Method | Endpoint |
|---|---|---|
| `hosts list` | GET | `/v1/hosts` |
| `hosts get <hostname>` | GET | `/v1/hosts?filter={hostname}` |
| `hosts mute <hostname>` | POST | `/v1/host/{hostname}/mute` |

Notes: `get` is implemented as a filtered `list` returning the first result.

## Traces / APM (v2)

| Command | Method | Endpoint |
|---|---|---|
| `traces search --query <q>` | POST | `/v2/spans/events/search` |
| `traces services` | GET | `/v2/apm/services?filter[env]=...` |

Notes: `search` prepends `service:{svc}` to the query when `--service` is given. `services` uses the v2 APM endpoint with `filter[env]` (defaults to `*` for all environments). `--search` filters service names client-side. `--env` filters server-side.

## Incidents (v2)

| Command | Method | Endpoint |
|---|---|---|
| `incidents list` | GET | `/v2/incidents` |
| `incidents get <id>` | GET | `/v2/incidents/{id}` |
| `incidents create` | POST | `/v2/incidents` |
| `incidents update <id>` | PATCH | `/v2/incidents/{id}` |

Notes: `create` wraps the body in JSON:API format (`{"data": {"type": "incidents", ...}}`). Commander is expressed as a `relationships.commander_user.data` object.

## SLOs (v1)

| Command | Method | Endpoint |
|---|---|---|
| `slo list` | GET | `/v1/slo` |
| `slo get <id>` | GET | `/v1/slo/{id}` |
| `slo history <id>` | GET | `/v1/slo/{id}/history?from_ts=...&to_ts=...` |

Notes: `get` and `history` unwrap the `{"data": {...}}` envelope.

## Auth / Validation

| Command | Method | Endpoint |
|---|---|---|
| `org test` | GET | `/v1/validate` |

## Raw API escape hatch

| Command | Method | Endpoint |
|---|---|---|
| `api [METHOD] <path>` | caller-chosen | caller-chosen (relative to `/api`) |

Notes: `api` is the escape hatch for endpoints the typed commands don't wrap
(e.g. server-side percentile aggregation via `POST /v2/spans/analytics/aggregate`,
facet discovery). It reuses the same credential/site resolution and
`classifyHTTPError` taxonomy as every typed command, so secrets are never
exposed and failures arrive as the usual `{error, fixable_by, hint}` contract.
A leading `/api` in the path is stripped (the base already ends in `/api`).
Reads (GET/HEAD, plus POST to `*/search` or `*/aggregate`) run by default;
other mutating requests require `--allow-write`. `--print-request` shows the
exact request (credentials redacted) without sending it — useful for telling a
CLI construction bug apart from a genuine API rejection. The response body is
passed through faithfully (no null-pruning). Pagination is not auto-followed:
pass the next cursor in your own follow-up `--body`/`--query`.

## Common Patterns

- **Auth headers**: All requests include `DD-API-KEY` and `DD-APPLICATION-KEY`.
- **Base URL**: `https://api.{site}/api` — path includes version (e.g. `/v1/monitor`).
- **Time params**: v1 endpoints use unix epoch seconds. v2 endpoints use RFC3339 strings.
- **Pagination**: List commands return the first page. Cursor metadata is surfaced in NDJSON output as `{"@pagination": {"has_more": true, "next_cursor": "..."}}` for logs and traces. Hosts surface `total_items` when truncated. Cursors are not auto-followed.
