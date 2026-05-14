package mockdd

import "time"

var monitors = []map[string]any{
	{"id": 1001, "name": "High CPU on checkout-service", "type": "metric alert", "overall_state": "alert", "query": "avg(last_5m):avg:system.cpu.user{service:checkout} > 90", "message": "CPU usage is critically high on checkout-service", "tags": []string{"service:checkout", "env:production", "team:platform"}, "muted": false, "priority": 1, "last_triggered_ts": 1711893600, "created": "2025-11-01T08:00:00Z", "modified": "2026-03-15T14:30:00Z"},
	{"id": 1002, "name": "Memory usage on search-service", "type": "metric alert", "overall_state": "warn", "query": "avg(last_10m):avg:system.mem.used{service:search} > 80", "message": "Memory usage elevated on search-service", "tags": []string{"service:search", "env:production", "team:search"}, "muted": false, "priority": 2, "last_triggered_ts": 1711980000, "created": "2025-12-01T10:00:00Z", "modified": "2026-03-20T09:00:00Z"},
	{"id": 1003, "name": "Error rate on payment-api", "type": "metric alert", "overall_state": "ok", "query": "sum(last_5m):sum:http.errors{service:payment-api} > 50", "message": "Error rate returned to normal", "tags": []string{"service:payment-api", "env:production", "team:payments"}, "muted": false, "priority": 1, "created": "2025-10-15T12:00:00Z", "modified": "2026-03-28T16:00:00Z"},
	{"id": 1004, "name": "No data from inventory-worker", "type": "service check", "overall_state": "no_data", "query": "\"datadog.agent.up\".over(\"service:inventory-worker\")", "message": "inventory-worker has stopped reporting", "tags": []string{"service:inventory-worker", "env:production", "team:warehouse"}, "muted": true, "priority": 3, "last_triggered_ts": 1711893600, "created": "2026-01-10T14:00:00Z", "modified": "2026-03-30T11:00:00Z"},
	{"id": 1005, "name": "Latency on gateway-api", "type": "metric alert", "overall_state": "ok", "query": "avg(last_5m):avg:http.request.duration{service:gateway-api} > 500", "message": "Gateway API latency is within acceptable range", "tags": []string{"service:gateway-api", "env:production", "team:platform"}, "muted": false, "priority": 2, "created": "2026-02-01T09:00:00Z", "modified": "2026-03-31T08:00:00Z"},
}

var events = []map[string]any{
	{"id": 5001, "id_str": "5001", "title": "Deploy: checkout-service v2.14.3", "text": "Deployed checkout-service v2.14.3 to production", "date_happened": 0, "source_type_name": "deploy", "tags": []string{"service:checkout", "env:production"}, "priority": "normal", "alert_type": "info", "url": "https://app.datadoghq.com/event/event?id=5001"},
	{"id": 5002, "id_str": "5002", "title": "Alert: High CPU on checkout-service", "text": "Monitor 1001 triggered: CPU > 90%", "date_happened": 0, "source_type_name": "monitor", "tags": []string{"service:checkout", "env:production"}, "priority": "normal", "alert_type": "error", "url": "https://app.datadoghq.com/event/event?id=5002"},
	{"id": 5003, "id_str": "5003", "title": "Scaling: search-service replicas 3→5", "text": "Auto-scaled search-service from 3 to 5 replicas", "date_happened": 0, "source_type_name": "autoscaling", "tags": []string{"service:search", "env:production"}, "priority": "normal", "alert_type": "info"},
}

var hosts = []map[string]any{
	{"name": "ip-10-0-1-101.ec2.internal", "aliases": []string{"checkout-1"}, "apps": []string{"agent", "ntp"}, "is_muted": false, "up": true, "sources": []string{"aws"}, "last_reported_time": 0},
	{"name": "ip-10-0-1-102.ec2.internal", "aliases": []string{"checkout-2"}, "apps": []string{"agent", "ntp"}, "is_muted": false, "up": true, "sources": []string{"aws"}, "last_reported_time": 0},
	{"name": "ip-10-0-2-201.ec2.internal", "aliases": []string{"search-1"}, "apps": []string{"agent", "ntp"}, "is_muted": true, "mute_timeout": 0, "up": true, "sources": []string{"aws"}, "last_reported_time": 0},
	{"name": "ip-10-0-3-301.ec2.internal", "aliases": []string{"payment-1"}, "apps": []string{"agent", "ntp"}, "is_muted": false, "up": true, "sources": []string{"aws"}, "last_reported_time": 0},
	{"name": "ip-10-0-4-401.ec2.internal", "aliases": []string{"gateway-1"}, "apps": []string{"agent"}, "is_muted": false, "up": false, "sources": []string{"aws"}, "last_reported_time": 0},
}

// incidentCommanderID is a fixture UUID used to populate the JSON:API
// `included` array consistently across the list and per-ID endpoints.
const incidentCommanderID = "11111111-2222-3333-4444-555555555555"

var incidentCommander = map[string]any{
	"id":   incidentCommanderID,
	"type": "users",
	"attributes": map[string]any{
		"handle": "alice@example.com",
		"name":   "Alice Engineer",
		"email":  "alice@example.com",
	},
}

func incidentWithCommander(id, title, state, severity string, customerImpacted bool, publicID int64, created string) map[string]any {
	return map[string]any{
		"id":   id,
		"type": "incidents",
		"attributes": map[string]any{
			"title":             title,
			"state":             state,
			"severity":          severity,
			"customer_impacted": customerImpacted,
			"public_id":         publicID,
			"created":           created,
		},
		"relationships": map[string]any{
			"commander_user": map[string]any{
				"data": map[string]any{"id": incidentCommanderID, "type": "users"},
			},
		},
	}
}

var incidents = []map[string]any{
	incidentWithCommander("inc-a1b2c3d4", "Elevated error rate on checkout-service", "active", "SEV-2", true, 102, "2026-03-31T14:00:00Z"),
	incidentWithCommander("inc-e5f6g7h8", "Search latency degradation", "stable", "SEV-3", false, 103, "2026-03-30T10:00:00Z"),
	incidentWithCommander("inc-i9j0k1l2", "Payment gateway timeout", "resolved", "SEV-1", true, 104, "2026-03-28T08:00:00Z"),
}

var slos = []map[string]any{
	{"id": "slo-aaa111", "name": "Checkout Availability", "type": "monitor", "description": "99.9% availability for checkout flow", "tags": []string{"service:checkout", "team:platform"}, "thresholds": []map[string]any{{"timeframe": "30d", "target": 99.9, "warning": 99.95}}, "overall_status": map[string]any{"status": 99.92, "error_budget_remaining": 0.02}},
	{"id": "slo-bbb222", "name": "Search P99 Latency", "type": "metric", "description": "P99 latency under 200ms for search", "tags": []string{"service:search", "team:search"}, "thresholds": []map[string]any{{"timeframe": "7d", "target": 99.0}}, "overall_status": map[string]any{"status": 98.5, "error_budget_remaining": -0.5}},
	{"id": "slo-ccc333", "name": "Payment Success Rate", "type": "monitor", "description": "99.95% payment success rate", "tags": []string{"service:payment-api", "team:payments"}, "thresholds": []map[string]any{{"timeframe": "30d", "target": 99.95, "warning": 99.97}}, "overall_status": map[string]any{"status": 99.98, "error_budget_remaining": 0.6}},
}

var services = []string{
	"checkout-service",
	"search-service",
	"payment-api",
	"gateway-api",
	"inventory-worker",
	"notification-service",
	"user-service",
	"analytics-pipeline",
}

var logMessages = []struct {
	Service string
	Status  string
	Message string
}{
	{"checkout-service", "error", "connection timeout to payment-api: dial tcp 10.0.3.301:8443: i/o timeout"},
	{"checkout-service", "error", "failed to process order: inventory reservation expired"},
	{"checkout-service", "warn", "slow database query: 2340ms on orders.find_by_user"},
	{"checkout-service", "info", "order completed: items=3 total=149.97"},
	{"search-service", "error", "elasticsearch cluster health: red, 2 unassigned shards"},
	{"search-service", "warn", "query took 850ms, threshold 200ms: query='winter jackets size:L'"},
	{"search-service", "info", "index rebuild completed: products_v4, 2.3M docs"},
	{"payment-api", "error", "stripe webhook signature verification failed"},
	{"payment-api", "info", "payment processed: amount=49.99 currency=GBP method=card"},
	{"gateway-api", "warn", "rate limit approaching: 4800/5000 requests in window"},
	{"gateway-api", "info", "health check passed: all upstream services healthy"},
	{"inventory-worker", "error", "failed to sync warehouse stock: connection refused"},
}

func nowUnix() int64 {
	return time.Now().Unix()
}

func nowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}
