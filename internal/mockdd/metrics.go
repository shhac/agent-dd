package mockdd

import (
	"math/rand"
	"net/http"
	"strings"
	"time"
)

func handleMetricQuery(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	// Mirror the real DD behavior: malformed queries return HTTP 200 with
	// status="error" and the reason in `error`, not a 4xx. The "fail-query"
	// sentinel lets integration tests exercise the QueryMetrics error path.
	if strings.Contains(query, "fail-query") {
		writeJSON(w, 200, map[string]any{
			"status":  "error",
			"error":   "query parse error: unknown function 'fail-query'",
			"message": "Unable to parse the query",
			"series":  []any{},
		})
		return
	}

	now := time.Now()
	points := make([][]float64, 12)
	for i := range 12 {
		ts := float64(now.Add(-time.Duration(12-i) * 5 * time.Minute).Unix())
		points[i] = []float64{ts, 40 + rand.Float64()*30}
	}

	writeJSON(w, 200, map[string]any{
		"status":    "ok",
		"res_type":  "time_series",
		"query":     query,
		"from_date": now.Add(-time.Hour).UnixMilli(),
		"to_date":   now.UnixMilli(),
		"series": []map[string]any{
			{
				"metric":      query,
				"display_name": query,
				"scope":       "env:mock",
				"tag_set":     []string{"env:mock"},
				"pointlist":   points,
				"interval":    300,
				"length":      int64(len(points)),
				"aggr":        "avg",
				"start":       now.Add(-time.Hour).UnixMilli(),
				"end":         now.UnixMilli(),
				"query_index": 0,
			},
		},
	})
}

func handleMetricList(w http.ResponseWriter, r *http.Request) {
	// /v2/metrics has no documented `filter[metric]` server-side filter;
	// agent-dd does substring matching client-side after we return everything.
	metrics := []string{
		"system.cpu.user", "system.cpu.system", "system.cpu.idle",
		"system.mem.used", "system.mem.free", "system.mem.cached",
		"system.disk.used", "system.disk.free",
		"http.requests", "http.errors", "http.request.duration",
	}

	data := make([]map[string]any, 0, len(metrics))
	for _, m := range metrics {
		data = append(data, map[string]any{"id": m, "type": "metrics"})
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func handleMetricMetadata(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/metrics/")
	// Real /v1/metrics/{name} response does NOT echo the metric name; the
	// client sets it from the request arg.
	writeJSON(w, 200, map[string]any{
		"type":            "gauge",
		"unit":            "percent",
		"description":     "Mock metric: " + name,
		"integration":     "mock",
		"per_unit":        "second",
		"short_name":      "mock " + name,
		"statsd_interval": 15,
	})
}
