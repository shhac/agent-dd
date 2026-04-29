package mockdd

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// NewHandler returns an http.Handler that simulates Datadog API endpoints.
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
	json.NewEncoder(w).Encode(data)
}

// --- Validate ---

func handleValidate(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"valid": true})
}

// --- Monitors ---

func handleMonitors(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/search") {
		handleMonitorSearch(w, r)
		return
	}
	writeJSON(w, 200, monitors)
}

func handleMonitorSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	results := make([]map[string]any, 0)
	for _, m := range monitors {
		name, _ := m["name"].(string)
		if query == "" || strings.Contains(strings.ToLower(name), strings.ToLower(query)) {
			results = append(results, m)
		}
	}
	writeJSON(w, 200, map[string]any{"monitors": results})
}

func handleMonitorByID(w http.ResponseWriter, r *http.Request) {
	// Handle /api/v1/monitor/{id}, /api/v1/monitor/search, /api/v1/monitor/{id}/mute, etc.
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/monitor/")
	if path == "search" {
		handleMonitorSearch(w, r)
		return
	}

	parts := strings.Split(path, "/")
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		writeJSON(w, 400, map[string]any{"errors": []string{"invalid monitor ID"}})
		return
	}

	for _, m := range monitors {
		if mid, _ := m["id"].(int); mid == id {
			writeJSON(w, 200, m)
			return
		}
	}
	writeJSON(w, 404, map[string]any{"errors": []string{"Monitor not found"}})
}

// --- Downtimes ---

var activeDowntimes = make([]map[string]any, 0)
var downtimeCounter int

func handleDowntimes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		downtimeCounter++
		dtID := fmt.Sprintf("dt-%06d", downtimeCounter)

		data, _ := body["data"].(map[string]any)
		attrs, _ := data["attributes"].(map[string]any)

		dt := map[string]any{
			"id":   dtID,
			"type": "downtime",
			"attributes": map[string]any{
				"status":  "active",
				"message": attrs["message"],
				"scope":   attrs["scope"],
			},
		}
		activeDowntimes = append(activeDowntimes, dt)
		writeJSON(w, 200, map[string]any{"data": dt})
		return
	}

	// GET — filter by monitor_id and status
	monitorFilter := r.URL.Query().Get("filter[monitor_id]")
	statusFilter := r.URL.Query().Get("filter[status]")

	results := make([]map[string]any, 0)
	for _, dt := range activeDowntimes {
		attrs, _ := dt["attributes"].(map[string]any)
		scope, _ := attrs["scope"].(string)
		status, _ := attrs["status"].(string)

		if statusFilter != "" && status != statusFilter {
			continue
		}
		if monitorFilter != "" && !strings.Contains(scope, "monitor_id:"+monitorFilter) {
			continue
		}
		results = append(results, dt)
	}
	writeJSON(w, 200, map[string]any{"data": results})
}

func handleDowntimeByID(w http.ResponseWriter, r *http.Request) {
	dtID := strings.TrimPrefix(r.URL.Path, "/api/v2/downtime/")
	if r.Method == http.MethodDelete {
		for i, dt := range activeDowntimes {
			if id, _ := dt["id"].(string); id == dtID {
				activeDowntimes = append(activeDowntimes[:i], activeDowntimes[i+1:]...)
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		writeJSON(w, 404, map[string]any{"errors": []string{"Downtime not found"}})
		return
	}
	writeJSON(w, 405, map[string]any{"errors": []string{"Method not allowed"}})
}

// --- Logs ---

func handleLogSearch(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	json.NewDecoder(r.Body).Decode(&body)

	filter, _ := body["filter"].(map[string]any)
	query, _ := filter["query"].(string)

	page, _ := body["page"].(map[string]any)
	limit := 50
	if l, ok := page["limit"].(float64); ok {
		limit = int(l)
	}

	entries := make([]map[string]any, 0, limit)
	for i := range limit {
		msg := logMessages[i%len(logMessages)]

		if query != "" {
			q := strings.ToLower(query)
			match := strings.Contains(strings.ToLower(msg.Service), q) ||
				strings.Contains(strings.ToLower(msg.Status), q) ||
				strings.Contains(strings.ToLower(msg.Message), q) ||
				strings.Contains(q, "service:"+strings.ToLower(msg.Service)) ||
				strings.Contains(q, "status:"+strings.ToLower(msg.Status))
			if !match {
				continue
			}
		}

		entries = append(entries, map[string]any{
			"id":   fmt.Sprintf("log-%08d", rand.Intn(99999999)),
			"type": "log",
			"attributes": map[string]any{
				"timestamp": time.Now().Add(-time.Duration(i) * time.Minute).Format(time.RFC3339),
				"service":   msg.Service,
				"status":    msg.Status,
				"message":   msg.Message,
				"host":      hosts[i%len(hosts)]["name"],
			},
		})
		if len(entries) >= limit {
			break
		}
	}

	writeJSON(w, 200, map[string]any{
		"data": entries,
		"meta": map[string]any{"page": map[string]any{"after": ""}},
	})
}

func handleLogAggregate(w http.ResponseWriter, r *http.Request) {
	buckets := []map[string]any{
		{"by": map[string]any{"service": "checkout-service", "status": "error"}, "computes": map[string]any{"c0": 142}},
		{"by": map[string]any{"service": "search-service", "status": "warn"}, "computes": map[string]any{"c0": 87}},
		{"by": map[string]any{"service": "payment-api", "status": "error"}, "computes": map[string]any{"c0": 23}},
		{"by": map[string]any{"service": "gateway-api", "status": "info"}, "computes": map[string]any{"c0": 5420}},
		{"by": map[string]any{"service": "inventory-worker", "status": "error"}, "computes": map[string]any{"c0": 56}},
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"buckets": buckets}})
}

// --- Metrics ---

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
				"metric": r.URL.Query().Get("query"),
				"tags":   []string{"env:production"},
				"points": points,
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

// --- Events ---

func handleEventList(w http.ResponseWriter, r *http.Request) {
	now := nowUnix()
	result := make([]map[string]any, len(events))
	for i, e := range events {
		copy := make(map[string]any)
		for k, v := range e {
			copy[k] = v
		}
		copy["date_happened"] = now - int64((len(events)-i)*600)
		result[i] = copy
	}
	writeJSON(w, 200, map[string]any{"events": result})
}

func handleEventByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/events/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, 400, map[string]any{"errors": []string{"invalid event ID"}})
		return
	}
	for _, e := range events {
		if eid, _ := e["id"].(int); int64(eid) == id {
			writeJSON(w, 200, map[string]any{"event": e})
			return
		}
	}
	writeJSON(w, 404, map[string]any{"errors": []string{"Event not found"}})
}

// --- Hosts ---

func handleHosts(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("filter")
	now := nowUnix()

	result := make([]map[string]any, 0)
	for _, h := range hosts {
		copy := make(map[string]any)
		for k, v := range h {
			copy[k] = v
		}
		copy["last_reported_time"] = now - int64(rand.Intn(300))

		name, _ := h["name"].(string)
		if filter != "" && !strings.Contains(name, filter) {
			continue
		}
		result = append(result, copy)
	}
	writeJSON(w, 200, map[string]any{
		"host_list":      result,
		"total_returned": len(result),
		"total_matching": len(result),
	})
}

func handleHostMute(w http.ResponseWriter, r *http.Request) {
	// POST /api/v1/host/{hostname}/mute
	writeJSON(w, 200, map[string]any{"action": "Muted", "hostname": "mocked"})
}

// --- Traces ---

func handleTraceSearch(w http.ResponseWriter, r *http.Request) {
	traceServices := []string{"checkout-service", "payment-api", "search-service", "gateway-api"}
	resources := []string{"POST /api/v1/orders", "GET /api/v1/search", "POST /api/v1/payments", "GET /api/v1/health"}

	data := make([]map[string]any, 10)
	for i := range 10 {
		svc := traceServices[i%len(traceServices)]
		status := "ok"
		errVal := 0
		if i%4 == 0 {
			status = "error"
			errVal = 1
		}
		data[i] = map[string]any{
			"type": "spans",
			"attributes": map[string]any{
				"trace_id": fmt.Sprintf("trace-%08x", rand.Intn(math.MaxInt32)),
				"span_id":  fmt.Sprintf("span-%08x", rand.Intn(math.MaxInt32)),
				"service":  svc,
				"name":     "http.request",
				"resource": resources[i%len(resources)],
				"duration": float64(rand.Intn(2000000000)),
				"status":   status,
				"error":    errVal,
				"start":    time.Now().Add(-time.Duration(i) * time.Second).UnixNano(),
			},
		}
	}

	writeJSON(w, 200, map[string]any{
		"data": data,
		"meta": map[string]any{"page": map[string]any{"after": ""}},
	})
}

func handleServiceList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"data": map[string]any{
			"attributes": map[string]any{
				"services": services,
			},
			"id":   "1",
			"type": "services_list",
		},
	})
}

// --- Incidents ---

func handleIncidents(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		data, _ := body["data"].(map[string]any)
		attrs, _ := data["attributes"].(map[string]any)
		inc := map[string]any{
			"id":   fmt.Sprintf("inc-%08x", rand.Intn(math.MaxInt32)),
			"type": "incidents",
			"attributes": map[string]any{
				"title":    attrs["title"],
				"status":   "active",
				"severity": "SEV-3",
				"created":  nowRFC3339(),
			},
		}
		writeJSON(w, 201, map[string]any{"data": inc})
		return
	}

	statusFilter := r.URL.Query().Get("filter[status]")
	results := make([]map[string]any, 0)
	for _, inc := range incidents {
		if statusFilter != "" {
			attrs, _ := inc["attributes"].(map[string]any)
			if status, _ := attrs["status"].(string); status != statusFilter {
				continue
			}
		}
		results = append(results, inc)
	}
	writeJSON(w, 200, map[string]any{"data": results})
}

func handleIncidentByID(w http.ResponseWriter, r *http.Request) {
	incID := strings.TrimPrefix(r.URL.Path, "/api/v2/incidents/")

	if r.Method == http.MethodPatch {
		writeJSON(w, 200, map[string]any{
			"data": map[string]any{
				"id":   incID,
				"type": "incidents",
				"attributes": map[string]any{
					"title":  "Updated incident",
					"status": "stable",
				},
			},
		})
		return
	}

	for _, inc := range incidents {
		if id, _ := inc["id"].(string); id == incID {
			writeJSON(w, 200, map[string]any{"data": inc})
			return
		}
	}
	writeJSON(w, 404, map[string]any{"errors": []string{"Incident not found"}})
}

// --- SLOs ---

func handleSLOList(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("query")
	results := make([]map[string]any, 0)
	for _, s := range slos {
		name, _ := s["name"].(string)
		if search == "" || strings.Contains(strings.ToLower(name), strings.ToLower(search)) {
			results = append(results, s)
		}
	}
	writeJSON(w, 200, map[string]any{"data": results})
}

func handleSLOByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/slo/")
	parts := strings.Split(path, "/")
	sloID := parts[0]

	if len(parts) > 1 && parts[1] == "history" {
		writeJSON(w, 200, map[string]any{
			"data": map[string]any{
				"overall": map[string]any{
					"sli_value":              99.92,
					"uptime":                 99.92,
					"error_budget_remaining": 0.02,
				},
			},
		})
		return
	}

	for _, s := range slos {
		if id, _ := s["id"].(string); id == sloID {
			writeJSON(w, 200, map[string]any{"data": s})
			return
		}
	}
	writeJSON(w, 404, map[string]any{"errors": []string{"SLO not found"}})
}
