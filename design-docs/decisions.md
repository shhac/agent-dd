# Design Decisions

## Single binary, zero runtime deps

Pure Go, `CGO_ENABLED=0`. No Datadog SDK — all API calls are hand-rolled HTTP with `net/http`. This keeps the binary small, cross-compilable, and free of transient dependency breakage. The only external dependency is `cobra` for CLI parsing.

## Structured JSON to stdout, errors to stderr

Designed for AI agent consumption. Stdout is always machine-parseable JSON (or NDJSON). Errors go to stderr as `{"error", "hint", "fixable_by"}` JSON. No human-friendly tables, colors, or progress bars.

## Token-efficient output

LLM context windows are finite. Default output is compact — e.g., monitors show only `id`, `name`, `status`, `type`. The `--full` flag returns the complete API response. This reduces token cost by ~80% for common triage queries.

## Null pruning

Datadog API responses contain many null/empty fields. `pruneNulls` recursively strips `nil` values from JSON output. Empty strings, `false`, and `0` are preserved (they carry meaning). This further reduces token waste without losing information.

## Error classification with `fixable_by`

Every error carries `fixable_by: agent | human | retry` so an AI agent can decide its next action without parsing error messages:

- **agent** — the agent made a bad request (wrong ID, bad query). It should fix its parameters and retry.
- **human** — credentials or permissions are wrong. The agent should stop and ask the human.
- **retry** — transient failure (rate limit, 5xx). The agent should wait and retry.

## macOS Keychain for credentials

API keys are sensitive. On macOS, credentials are stored in the system Keychain (service: `app.paulie.agent-dd`) rather than plaintext files. The credential index file (`~/.config/agent-dd/credentials.json`, mode `0600`) stores metadata and a `keychain_managed` flag. On non-darwin platforms, keys fall back to plaintext in the index file.

## Site-aware base URL

Datadog has multiple regional sites (`datadoghq.com`, `datadoghq.eu`, `us3/us5/ap1.datadoghq.com`). The base URL is computed as `"https://api." + site + "/api"`. This is stored per-org in config, so a single CLI instance can talk to multiple Datadog orgs on different sites.

## "Org" not "project"

Datadog's entity is an organization, not a project. The CLI uses `--org` / `-o` as the global flag, `DD_ORG` as the env var, and `org` as the subcommand group. This matches Datadog's own terminology.

## Client-side filtering where necessary

Some Datadog APIs don't support server-side filtering for all fields. In these cases, the CLI fetches the full list and filters client-side. Examples: `ListMonitors` status filtering (v1 API doesn't support it), `ListServices` `--search` substring filtering (the v2 `filter[env]` is server-side, but name search is client-side).

## `doAndDecode[T]` generic helper

Go generics allow a single function to handle the common pattern of "call API, unmarshal response body into typed struct." This eliminates boilerplate across the ~20 API functions while keeping return types explicit.

## Mute/unmute via Downtimes v2

Datadog deprecated the v1 `POST /v1/monitor/{id}/mute` and `/unmute` endpoints. In Datadog's model, muting a monitor is creating a downtime — there's no separate "mute" concept in v2.

Our CLI preserves the simple `monitors mute <id>` / `monitors unmute <id>` interface but implements it via the Downtimes v2 API:
- **Mute**: `POST /v2/downtime` with `scope: "monitor_id:{id}"` and `monitor_identifier: {monitor_id: id}`
- **Unmute**: `GET /v2/downtime?filter[monitor_id]=...&filter[status]=active` to find active downtimes, then `DELETE /v2/downtime/{id}` for each

Unmute only cancels **active** downtimes — scheduled future downtimes (e.g. planned maintenance windows) are left intact.

## Multi-get contract: `get <id>...`

Entity gets accept 1..N ids and emit one NDJSON line per input, in input order: either the record, or `{"@unresolved":{"id","reason","fixable_by","hint"?}}` for an id that couldn't be resolved (not found, bad id, rate-limited). This is interleaved (positional), not a trailing aggregate.

Single-get is just the one-element case — default output is NDJSON (one line), matching multi-get. Pass `--format json` to get the plain JSON object (useful for piping into `jq`).

Failure split: item-level misses go to stdout as `@unresolved` lines, exit 0. Command-level failures (auth, network, bad flags, zero args) go to stderr as `{"error",...}`, exit 1, empty stdout. Exit code signals whether the command ran, not whether every id resolved.

`--format json|yaml` collapses the stream to `{"data":[…], "@unresolved":[…]}` envelope.

## DI via package-level `ClientFactory`

Tests need to intercept client creation to inject `httptest.Server` URLs. A package-level `func() (*api.Client, error)` variable is the simplest DI mechanism that doesn't require interfaces or dependency injection frameworks. It's nil in production, set only during tests, and cleaned up via `t.Cleanup`.
