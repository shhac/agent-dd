package mockdd

import (
	"encoding/json"
	"net/http"
	"strings"
)

// server binds the per-request handlers to one store. Handlers are methods so
// that each NewHandler() call gets isolated state; see store.go.
type server struct {
	*store
}

// NewHandler returns an http.Handler that simulates Datadog API endpoints.
// Per-domain handlers live in sibling files (monitors.go, traces.go, etc.).
// Each call gets its own state — two handlers never share writes.
func NewHandler() http.Handler {
	s := &server{store: newStore()}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/validate", s.handleValidate)
	mux.HandleFunc("/api/v1/monitor", s.handleMonitors)
	mux.HandleFunc("/api/v1/monitor/", s.handleMonitorByID)
	mux.HandleFunc("/api/v2/downtime", s.handleDowntimes)
	mux.HandleFunc("/api/v2/downtime/", s.handleDowntimeByID)
	mux.HandleFunc("/api/v2/logs/events/search", s.handleLogSearch)
	mux.HandleFunc("/api/v2/logs/analytics/aggregate", s.handleLogAggregate)
	mux.HandleFunc("/api/v1/query", s.handleMetricQuery)
	mux.HandleFunc("/api/v2/metrics", s.handleMetricList)
	mux.HandleFunc("/api/v1/metrics/", s.handleMetricMetadata)
	mux.HandleFunc("/api/v1/events", s.handleEventList)
	mux.HandleFunc("/api/v1/events/", s.handleEventByID)
	mux.HandleFunc("/api/v1/hosts", s.handleHosts)
	mux.HandleFunc("/api/v1/host/", s.handleHostMute)
	mux.HandleFunc("/api/v2/spans/events/search", s.handleTraceSearch)
	mux.HandleFunc("/api/v2/apm/services", s.handleServiceList)
	mux.HandleFunc("/api/v2/incidents", s.handleIncidents)
	mux.HandleFunc("/api/v2/incidents/", s.handleIncidentByID)
	mux.HandleFunc("/api/v1/slo", s.handleSLOList)
	mux.HandleFunc("/api/v1/slo/", s.handleSLOByID)

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

// writeError emits Datadog's generic error envelope. Every 4xx in this package
// uses the same single-string shape, so it lives in one place.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"errors": []string{msg}})
}

// methodNotAllowed keeps handlers strict about verbs. A handler that answers
// every method identically lets a write-path test pass against a mock that
// never implemented the write — green for entirely the wrong reason.
func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	writeError(w, 405, "Method not allowed")
}

// requireMethod is the guard for single-verb endpoints. Reports false (having
// already written the 405) when the caller used the wrong verb.
func requireMethod(w http.ResponseWriter, r *http.Request, allowed string) bool {
	if r.Method != allowed {
		methodNotAllowed(w, allowed)
		return false
	}
	return true
}

// decodeAttributes unwraps the JSON:API {"data":{"attributes":{...}}} envelope
// the v2 write endpoints use. A missing or malformed body yields an empty map,
// matching how the handlers previously tolerated one inline.
func decodeAttributes(r *http.Request) map[string]any {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	data, _ := body["data"].(map[string]any)
	attrs, _ := data["attributes"].(map[string]any)
	if attrs == nil {
		return map[string]any{}
	}
	return attrs
}

func (s *server) handleValidate(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, 200, map[string]any{"valid": true})
}
