# Architecture

## Request Lifecycle

Every CLI command follows the same path:

```
main.go → cobra dispatch → shared.WithClient(orgAlias, timeout, fn) → fn(ctx, client)
                                   │
                                   ├─ resolve credentials (env vars → org config → keychain)
                                   ├─ construct api.Client
                                   └─ on error: output.WriteError to stderr, return nil
```

Inside `fn`, the command calls an `api.Client` method, transforms the result (compact vs full view), and writes to stdout via `output.PrintJSON` or `shared.WritePaginatedList`.

## Key Abstractions

### Client DI (`shared.ClientFactory`)

`shared.ClientFactory` is a package-level `func() (*api.Client, error)`, nil in production. `WithClient` checks it before calling `NewClientFromOrg`. Tests override it via `shared.SetupMockServer`, which spins up an `httptest.Server` and injects a factory returning `api.NewTestClient(srv.URL, ...)`.

### Credential Resolution (`shared.NewClientFromOrg`)

Resolution order:

1. **Env vars** — if `DD_API_KEY` + `DD_APP_KEY` are set and no org alias was given, use them directly. `DD_SITE` defaults to `datadoghq.com`.
2. **Org alias** — from `--org` flag, `DD_ORG` env var, or `config.DefaultOrg` in `~/.config/agent-dd/config.json`.
3. **Credential lookup** — `credential.Get(alias)` reads the credential index (`~/.config/agent-dd/credentials.json`). If `keychain_managed: true`, retrieves keys via macOS `security` CLI (service: `app.paulie.agent-dd`). Otherwise uses plaintext from the index file (permissions `0600`).
4. **Site config** — reads `Organization.Site` from config for the alias. Falls back to `datadoghq.com`.

### HTTP Client (`api.Client`)

```go
Client {
    baseURL string       // "https://api." + site + "/api"
    apiKey  string
    appKey  string
    http    *http.Client
}
```

`c.do(ctx, method, path, body)` constructs the full URL as `baseURL + path` (path includes the version segment, e.g. `/v1/monitor`). Sets `DD-API-KEY` and `DD-APPLICATION-KEY` headers. Returns `json.RawMessage` on success; classifies HTTP errors on failure.

`doAndDecode[T]` is a generic wrapper that calls `do` and unmarshals into `*T`.

### Error Classification (`classifyHTTPError`)

All errors are `*errors.APIError` with a `fixable_by` field:

| HTTP Status | fixable_by | Meaning |
|---|---|---|
| 401 | `human` | Bad API/app keys |
| 403 | `human` | Insufficient permissions |
| 404 | `agent` | Wrong ID — use `list` to find valid ones |
| 429 | `retry` | Rate limited — wait and retry |
| 5xx | `retry` | Datadog-side error |
| other 4xx | `agent` | Bad request — fix parameters |

```go
type APIError struct {
    Message   string    `json:"error"`
    Hint      string    `json:"hint,omitempty"`
    FixableBy FixableBy `json:"fixable_by"`
    Cause     error     `json:"-"`
}
```

Constructors: `New`, `Newf`, `Wrap`. Chainable setters: `WithHint`, `WithCause`.

### Output (`output` package)

- **Formats**: `json` (indented), `jsonl`/`ndjson` (one object per line), `yaml`. Lists default to NDJSON, single items default to JSON.
- **Null pruning**: `pruneNulls` recursively removes keys with `nil` values from maps. Empty strings, `false`, `0` are preserved.
- **Error output**: `WriteError` converts any error to `APIError` (wrapping non-APIError as `fixable_by: agent`) and writes JSON to stderr.
- **Paginated lists**: `shared.WritePaginatedList` handles both JSON (wrapped in `{"data": [...], "pagination": {...}}`) and NDJSON (item-per-line with trailing pagination record).

### Command Registration Pattern

Each command group lives in its own package under `internal/cli/`:

```go
func Register(parent *cobra.Command, globals func() *shared.GlobalFlags) {
    cmd := &cobra.Command{Use: "monitors", Short: "..."}
    registerList(cmd, globals)
    registerGet(cmd, globals)
    // ...
    registerLLMHelp(cmd)
    parent.AddCommand(cmd)
}
```

`globals` is a closure from `root.go` that captures flag variables and returns `*shared.GlobalFlags` at call time (after cobra has parsed flags).

## File Layout Rationale

```
internal/api/        → HTTP client + all Datadog API calls (no CLI concerns)
internal/cli/        → Cobra commands, flag parsing, output formatting
internal/cli/shared/ → DI, credential resolution, time parsing, test helpers
internal/config/     → App config file (org → site mapping, default org)
internal/credential/ → Credential storage (index file + keychain)
internal/errors/     → APIError type with fixable_by classification
internal/output/     → JSON/NDJSON formatting, null pruning, error serialization
```

The API layer has zero dependency on CLI concerns. Commands depend on `api`, `shared`, and `output`. This makes the API layer independently testable and reusable.
