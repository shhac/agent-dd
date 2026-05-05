package mockdd

import (
	"encoding/json"
	"net/http"
)

// NewHandler returns an http.Handler that simulates Datadog API endpoints.
// Per-domain handlers live in sibling files (monitors.go, traces.go, etc.).
func NewHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/validate", handleValidate)
	mux.HandleFunc("/api/v1/monitor", handleMonitors)
	mux.HandleFunc("/api/v1/monitor/", handleMonitorByID)
	mux.HandleFunc("/api/v2/downtime", handleDowntimes)
	mux.HandleFunc("/api/v2/downtime/", handleDowntimeByID)
	mux.HandleFunc("/api/v2/logs/events/search", handleLogSearch)
	mux.HandleFunc("/api/v2/logs/analytics/aggregate", handleLogAggregate)
	mux.HandleFunc("/api/v1/query", handleMetricQuery)
	mux.HandleFunc("/api/v2/metrics", handleMetricList)
	mux.HandleFunc("/api/v1/metrics/", handleMetricMetadata)
	mux.HandleFunc("/api/v1/events", handleEventList)
	mux.HandleFunc("/api/v1/events/", handleEventByID)
	mux.HandleFunc("/api/v1/hosts", handleHosts)
	mux.HandleFunc("/api/v1/host/", handleHostMute)
	mux.HandleFunc("/api/v2/spans/events/search", handleTraceSearch)
	mux.HandleFunc("/api/v2/apm/services", handleServiceList)
	mux.HandleFunc("/api/v2/incidents", handleIncidents)
	mux.HandleFunc("/api/v2/incidents/", handleIncidentByID)
	mux.HandleFunc("/api/v1/slo", handleSLOList)
	mux.HandleFunc("/api/v1/slo/", handleSLOByID)

	return authMiddleware(mux)
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("DD-API-KEY") == "" || r.Header.Get("DD-APPLICATION-KEY") == "" {
			writeJSON(w, 401, map[string]any{"errors": []string{"Authentication failed"}})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func handleValidate(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"valid": true})
}
