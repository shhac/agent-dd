# agent-dd

Datadog triage CLI for AI agents. Focused on investigation workflows — logs, metrics, monitors, traces, incidents, SLOs — not full Datadog administration.

## Design Docs

- [Architecture](design-docs/architecture.md) — request lifecycle, credential resolution, client DI, error classification, output formatting
- [Design Decisions](design-docs/decisions.md) — rationale behind key choices
- [API Mapping](design-docs/api-mapping.md) — how CLI commands map to Datadog API endpoints

## Dev Workflow

```bash
make build          # Build binary
make test           # Run all tests
make vet            # Go vet
make dev ARGS="monitors list --status alert"  # Run without building
```

## Testing

Tests use `shared.SetupMockServer()` which creates an `httptest.Server` and injects it via `shared.ClientFactory`. Tests verify:
- Correct API paths and methods
- Request headers (DD-API-KEY, DD-APPLICATION-KEY)
- Request body structure
- Error classification for HTTP status codes
- Time parsing (relative, RFC3339, epoch)

## Releasing

Uses goreleaser. Tag a version and push:
```bash
git tag v0.1.0
git push --tags
goreleaser release
```

Also distributed via homebrew tap (shhac/tap).

## Environment Variables

- `DD_API_KEY` + `DD_APP_KEY` — direct credential fallback (skips org config)
- `DD_SITE` — Datadog site (with env var auth)
- `DD_ORG` — organization alias (like `--org` flag)

## What This Tool Does NOT Do

- Dashboard/widget management
- Synthetic tests
- User/role management
- Log pipeline configuration
- Metric ingestion/submission
- Notebook management
