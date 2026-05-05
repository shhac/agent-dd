package mockdd

import (
	"math/rand"
	"net/http"
	"strings"
	"time"
)

func handleMetricQuery(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	points := make([][]float64, 12)
	for i := range 12 {
		ts := float64(now.Add(-time.Duration(12-i) * 5 * time.Minute).Unix())
		points[i] = []float64{ts, 40 + rand.Float64()*30}
	}

	writeJSON(w, 200, map[string]any{
		"status": "ok",
		"series": []map[string]any{
			{
				"metric":    r.URL.Query().Get("query"),
				"scope":     "env:mock",
				"tag_set":   []string{"env:mock"},
				"pointlist": points,
				"interval":  300,
				"length":    len(points),
				"aggr":      "avg",
			},
		},
	})
}

func handleMetricList(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("filter[metric]")
	metrics := []string{
		"system.cpu.user", "system.cpu.system", "system.cpu.idle",
		"system.mem.used", "system.mem.free", "system.mem.cached",
		"system.disk.used", "system.disk.free",
		"http.requests", "http.errors", "http.request.duration",
	}

	data := make([]map[string]any, 0)
	for _, m := range metrics {
		if search == "" || strings.Contains(m, search) {
			data = append(data, map[string]any{"id": m, "type": "metrics"})
		}
	}
	writeJSON(w, 200, map[string]any{"data": data})
}

func handleMetricMetadata(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/metrics/")
	writeJSON(w, 200, map[string]any{
		"metric":      name,
		"type":        "gauge",
		"unit":        "percent",
		"description": "Mock metric: " + name,
	})
}
