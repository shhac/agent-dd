# Datadog Query Syntax Reference

## Log queries

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

### Tips

- Start broad with `logs facets` to see which services/hosts/statuses have volume
- Then add filters to `logs search` to drill into specific results
- Combine multiple filters to narrow progressively: `service:web status:error @http.status_code:>500`

## Metric queries

Used by `metrics query`.

```
avg:system.cpu.user{host:web-1}                    # basic: aggregation:metric{filter}
sum:http.requests{env:prod} by {service}            # grouping: split by tag
max:system.disk.used{*}                             # all hosts
avg:app.request.duration{service:api,env:prod}      # multiple filters (AND-ed)
```

Aggregations: `avg`, `sum`, `min`, `max`, `count`.

## Monitor queries

Used by `monitors create --query` and `monitors update --query`. **Not the same
as a metric query** — a monitor query adds an evaluation window and a threshold
comparison, and the grammar differs per monitor `--type`. Always `--dry-run`
first; Datadog parses the query with the same engine that would run it.

### `--type "metric alert"`

```
time_aggr(time_window):space_aggr:metric{tags} [by {key}] operator threshold
```

```
avg(last_5m):avg:system.cpu.user{service:web} > 90
sum(last_10m):sum:http.errors{env:prod} > 50
avg(last_5m):avg:system.mem.used{*} by {host} > 90       # multi-alert, one per host
max(last_15m):max:app.request.duration{service:api} >= 2000
```

- `time_aggr` — how points combine over the window: `avg`, `sum`, `min`, `max`
- `time_window` — `last_1m`, `last_5m`, `last_10m`, `last_15m`, `last_30m`, `last_1h`, `last_4h`, `last_1d`
- `space_aggr` — how series combine across sources: `avg`, `sum`, `min`, `max`
- `by {key}` — makes it a multi-alert: a separate notification per group
- `operator` — `>`, `>=`, `<`, `<=`, `==`

The threshold in the query should match `--threshold-critical`.

### `--type "log alert"`

```
logs("<log query>").index("<index>").rollup("<method>").by("<facet>").last("<window>") operator threshold
```

```
logs("service:web AND status:error").index("*").rollup("count").last("5m") > 10
logs("status:error").index("main").rollup("count").by("service").last("15m") > 100
```

The string inside `logs(...)` is ordinary log query syntax (see above).

### `--type "service check"`

```
"<check>".over(tags).last(count).by(group).count_by_status()
```

```
"datadog.agent.up".over("service:web").last(3).count_by_status()
"http.can_connect".over("instance:api").last(4).by("host").count_by_status()
```

`over(...)` is required. `last(count)` must be at least your largest threshold.

## Trace queries

Traces use the same log query syntax but with APM-specific facets.

```bash
agent-dd traces search --query "service:web-api @duration:>1000000000" --from now-30m
agent-dd traces search --query "status:error" --service web-api
agent-dd traces search --service web-api    # all traces for a service
```

Duration is in **nanoseconds** (1s = 1,000,000,000ns).

Common facets: `service`, `resource_name`, `@duration`, `status`, `@http.status_code`.
