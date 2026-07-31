# Plan: `monitors create`, `update` and `delete`

Status: **approved** — phase 0 in progress.

This is a working plan, not a durable design doc. Once implemented, the parts
worth keeping move into [decisions.md](../decisions.md) and
[api-mapping.md](../api-mapping.md), and this file is deleted.

## Why

Triage doesn't end at diagnosis. Once an agent has worked out what broke, the
next two questions a human asks are "how do we stop this happening again?" and
"how do we get visibility on it?" Both are answered by creating a monitor or
adjusting an existing one. Today `agent-dd` can tell you a monitor is firing,
mute it, and unmute it — but the moment you want to act on what you learned,
you fall out of the tool and into `agent-dd api POST /v1/monitor --allow-write`
with a hand-written JSON body.

That gap is the whole reason this is worth building: the diagnose → harden loop
should stay inside one tool with one credential path and one output format.

### Scope implications

`CLAUDE.md` and `skills/agent-dd/SKILL.md` both currently say this tool does
triage, "not full Datadog administration." Adding create/update moves that line
deliberately. The revised boundary is:

> Write operations that follow directly from an investigation are in scope.
> Datadog administration that doesn't (dashboards, users, roles, pipelines,
> synthetics) stays out.

That wording needs to land in `decisions.md` as part of phase 5, otherwise the
next person to read the scope statement will reasonably conclude this feature
was a mistake.

## API surface

Verified against current Datadog docs (2026-07):

| Operation | Endpoint | Notes |
|---|---|---|
| Create | `POST /v1/monitor` | `type` + `query` required. Needs `monitors_write`. |
| Edit | `PUT /v1/monitor/{id}` | Partial top-level fields accepted. Needs `monitors_write`. |
| Validate new | `POST /v1/monitor/validate` | No state change. `monitors_read` only. |
| Validate edit | `POST /v1/monitor/{id}/validate` | No state change. `monitors_read` only. |
| Delete | `DELETE /v1/monitor/{id}` | Optional `force` query param. Needs `monitors_write`. |

The validate endpoints are the find that shapes the design. They give a real
dry-run — Datadog itself confirms the query parses and the options are coherent
— without writing anything. That makes `--dry-run` a first-class flag rather
than a client-side approximation.

## Command surface

```
agent-dd monitors create --type <t> --query <q> --name <n>
                         [--message <m>] [--tag k:v]... [--priority N]
                         [--threshold-critical N] [--threshold-warning N]
                         [--notify-no-data] [--no-data-timeframe N]
                         [--renotify-interval N]
                         [--body @file|@-] [--dry-run]

agent-dd monitors update <id> [same field flags] [--dry-run]

agent-dd monitors delete <id> --yes [--force]
```

Typed flags cover the handful of options that actually come up after an
incident. This is deliberate: `SKILL.md` is the only reference an agent has, and
expecting it to reconstruct Datadog's `options` schema from memory is how you
get confidently malformed monitors. `--body @file|@-` remains the
full-fidelity escape hatch for everything else, reusing the `@file`/`@-`
convention already established by `agent-dd api --body`.

## The central design decision: `update` must not destroy what it doesn't model

`api.Monitor` (`internal/api/monitors.go:15`) models perhaps 15% of a real
Datadog monitor. A naive `update` that marshals that struct and PUTs it would
silently drop `restricted_roles`, most of `options`, `notify_audit`, composite
sub-monitor definitions — everything the CLI doesn't have a field for. Nested
`options` is the sharpest edge: supplying a partial `options` object on PUT is
widely treated as replacing the object wholesale rather than deep-merging it, so
`--renotify-interval 30` on a monitor with tuned thresholds could erase those
thresholds.

So `update` is a **read-modify-write over an untyped map**:

1. `GET /v1/monitor/{id}`, decode into `map[string]any`
2. Strip read-only keys — `overall_state`, `created`, `modified`,
   `last_triggered_ts`, `creator`, `org_id`, `matching_downtimes`, `deleted`
3. Apply only the flags the user actually set (`cmd.Flags().Changed`), merging
   into `options` key-by-key rather than replacing the object
4. `PUT` the merged map
5. Emit a before/after diff of just the changed fields

The payoff is that **the CLI never has to model Datadog's full monitor schema in
order to avoid clobbering fields it doesn't know about**, and the behaviour is
correct regardless of what Datadog's actual partial-update semantics turn out to
be. The diff output also gives an agent something concrete to report back to a
human: "critical threshold 5 → 20, nothing else changed."

## Write gating

**Decided: `create` and `update` are ungated.** No `--allow-write`-style flag on
either. `incidents create` (`internal/cli/incidents/incidents.go:99`) already
sets that precedent, and a gate an agent satisfies by appending `--yes` buys
nothing against an agent — it only costs ergonomics for the caller doing the
right thing. The real safety is `--dry-run` against Datadog's own validate
endpoints, plus the before/after diff on update.

The counter-argument, recorded because it's reasonable: `create` is additive and
reversible, whereas `update` silently changes existing production alerting, so
`update` alone could warrant a confirmation flag. It was weighed and rejected —
if the diff turns out not to be sufficient safety in practice, adding a gate
later is a one-line change, whereas removing one people have scripted around is
not.

The `api` command's `--allow-write` gate stays as-is; it exists because `api` is
an untyped hole where any method can reach any path. Typed commands are
self-describing and don't need the same treatment.

## `monitors delete`

Included: a create the tool can't undo is a half-feature, and cleaning up a
monitor that turned out to be noisy is part of the same loop as creating it.

Unlike create and update, delete **is** gated behind a required `--yes`. The
reasoning that makes gating pointless elsewhere doesn't apply here: create and
update leave the monitor readable and correctable afterwards, whereas delete is
the one operation in this set with no in-tool recovery path. `--yes` isn't
there to stop a determined agent — it's there so deletion can never be the
result of a half-constructed command line.

Datadog refuses to delete a monitor referenced by another resource (an SLO, or a
composite monitor) unless `force` is set. Rather than pass `--force` straight
through, the 400 should be surfaced with `fixable_by: agent` and a hint naming
`--force`, so an agent that hits the referenced-resource case has to make a
second, deliberate decision. That referential check is a genuine safety signal
and shouldn't be defaulted away.

## Prerequisite: mockdd cannot currently model a write

Development follows red-green with mockdd as the DI seam, which means mockdd has
to be trustworthy before any write path is built on it. Two problems block that.

### 1. Mock state is global, not per-server

Fixtures are package-level globals (`internal/mockdd/data.go:5`) and mutable
state like `activeDowntimes` (`internal/mockdd/downtimes.go:10`) is shared by
every server the test binary creates. `NewHandler()` builds a fresh mux, but the
handlers close over the same package vars. Demonstrated with a throwaway probe:

```
TestLeakA: creates a downtime          --- PASS
TestLeakB: fresh server, expects 0     --- FAIL: got 1
```

This is currently latent — nothing asserts downtime counts across tests. Once
monitors become mutable it becomes order-dependent cross-test contamination,
which is the failure mode that destroys trust in a suite and makes red-green
worthless, because you stop believing red.

### 2. Handlers ignore the HTTP method

`handleMonitors` does `writeJSON(w, 200, monitors)` unconditionally
(`internal/mockdd/monitors.go:14`). A `POST /v1/monitor` today returns 200 and a
list of monitors. Any first "red" test written against that would go green for
entirely the wrong reason.

### Fix

`NewHandler()` constructs a `server` struct holding its own store seeded from
the fixtures; handlers become methods on it; unrecognised methods return 405.
`NewHandler()`'s signature is unchanged, so the refactor is verified by *every
existing test still passing*, plus the leak probe flipping to green — which is
kept permanently as a regression test. It also fixes the dormant downtime bug.

## The red-green unit

Each capability runs through this loop. It's the repeating unit of work, and
each phase below is one or more passes of it.

1. **Red (compile)** — integration test in `mockdd/integration_test.go` driving
   a real `api.Client` against mockdd. Fails to build: no `CreateMonitor`.
2. **Red (behaviour)** — add the client method stub and the mockdd route. Now it
   compiles and fails on the assertion.
3. **Green** — implement client and handler until the round-trip passes.
4. **Sharpen** — request-shape assertions (path, method, exact body JSON) via a
   bespoke `shared.SetupMockServer` handler, plus error-path coverage. This
   split follows the convention already documented in `mockddtest.go`'s package
   comment: mockdd proves canonical wire shapes, bespoke handlers prove request
   shape and injected errors.
5. **Structure** — run `improve-code-structure`, apply its recommendations,
   re-run the suite.

For `--dry-run`, mockdd should model `/v1/monitor/validate` returning 400 for a
deliberately malformed query, so the failure path is covered by a real
round-trip rather than a hand-stubbed 400.

## Phases

| Phase | Content | Red-green anchor |
|---|---|---|
| **0** | mockdd per-server state, strict method dispatch | Leak probe fails → passes; all existing tests stay green |
| **1** | mockdd write routes; `CreateMonitor` / `UpdateMonitor` / validate on the client | Round-trip create → get |
| **2** | Pure `stripReadOnly` / `applyUpdates` merge functions | Table-driven, no server at all |
| **3** | `monitors create` + `--dry-run` | CLI test via `mockddtest.InstallClientFactory` |
| **4** | `monitors update` — read-modify-write + diff | Round-trip: create → update one field → get → assert nothing else moved |
| **5** | `monitors delete` + `--yes` / `--force` | Round-trip: create → delete → get returns 404; referenced monitor 400s without `--force` |
| **6** | Docs | — |

Phases 1–5 are the meaningful slice. Phase 6 is what makes it usable by an agent
that wasn't present for the implementation.

Delete lands last of the write commands deliberately: by then create and update
have already forced mockdd to hold real per-server monitor state, so the
create → delete → 404 round-trip is a genuine assertion rather than a stub
agreeing with itself. It also means every earlier phase's tests can create
throwaway monitors without a cleanup path existing yet — per-server state makes
that harmless.

Phase 2 is where the modularity pays. The genuinely risky logic — "merge these
changes without destroying options we never modelled" — becomes a pure function
over `map[string]any` with table tests and no HTTP anywhere. mockdd then only
has to prove the wire format is right, not the merge semantics. Those are two
different classes of bug and testing them through the same mechanism means
neither is tested well.

### Phase 6 doc surface

- `internal/cli/monitors/usage.go` — new commands in the `COMMANDS` block,
  worked examples, a note that `update` preserves unmodelled fields, and that
  `delete` needs `--yes` (plus `--force` for referenced monitors)
- `skills/agent-dd/SKILL.md` — a named "harden after diagnosis" workflow step,
  since this is the doc an agent actually reads
- `design-docs/api-mapping.md` — the five new endpoint rows
- `design-docs/decisions.md` — the scope expansion, the read-modify-write
  rationale, and why delete is gated when create and update aren't
- `CLAUDE.md` + `SKILL.md` — amend the "not administration" line
- `README.md` — command list

## Verification

Per phase, via the existing harnesses:

- Exact paths, methods, headers and request-body JSON for create and update
- `--dry-run` hits `/validate` and never the write path
- Read-modify-write issues `GET` then `PUT`, and untouched `options` subfields
  survive the round-trip
- `delete` without `--yes` performs no request at all
- Error classification: 400 bad query → `fixable_by: agent`; 403 missing
  `monitors_write` → `fixable_by: human`; 400 referenced-resource on delete →
  `fixable_by: agent` with a hint naming `--force`
- `make test` and `make vet` green at every phase boundary
- Manual smoke against a real org before release: create a throwaway monitor,
  get it, update one threshold, confirm nothing else moved, delete it

## Expected structural pressure

Named upfront so the `improve-code-structure` checkpoints aren't a surprise —
but deliberately **not** pre-built, because designing to the skill's predicted
output makes its checkpoints meaningless.

- **`monitors.go` splitting.** Already ~200 lines with five `register*`
  functions; create and update add roughly 20 more flags. Likely lands as
  `monitors.go` (Register + projection), `read.go`, `write.go`.
- **Shared flag struct.** `create` and `update` want nearly identical flag sets.
  Probably a `monitorFlags` struct with `bind(cmd)` and
  `changes(cmd) map[string]any`.
- **`resolveBody` hoisting.** Currently private in `apicmd`
  (`internal/cli/apicmd/api.go:141`); a second caller makes it a `shared` helper.
- **mockdd handler methods.** Phase 0 makes every handler a method on one
  struct, which will get large — the skill may push toward per-domain sub-stores.

## Cost note

Phase 0 delivers no user-facing feature and is a mechanical pass over ~10
handler files. It's the price of mockdd being trustworthy enough to drive
write-path development, and it pays down a real if currently dormant bug.
Skipping it means building write paths against a mock that cannot correctly
model a write. **Accepted** — proceeding with phase 0 first.
