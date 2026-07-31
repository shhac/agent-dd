package mockdd

import (
	"net/http"
	"strconv"
	"strings"
)

// Search never reaches here: the mux registers "/api/v1/monitor" as an exact
// pattern, so /api/v1/monitor/search resolves to the "/api/v1/monitor/" subtree
// and lands in handleMonitorByID, which dispatches it.
func (s *server) handleMonitors(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, 200, s.allMonitors())
	default:
		methodNotAllowed(w, http.MethodGet)
	}
}

func (s *server) handleMonitorSearch(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	query := r.URL.Query().Get("query")
	results := make([]map[string]any, 0)
	statusBucket := map[string]int{}
	mutedBucket := map[string]int{}
	for _, m := range s.allMonitors() {
		name, _ := m["name"].(string)
		// `*` is Datadog's match-all sentinel; treat it as such instead of
		// a literal substring (which matches nothing).
		if query == "" || query == "*" || strings.Contains(strings.ToLower(name), strings.ToLower(query)) {
			results = append(results, m)
			if state, _ := m["overall_state"].(string); state != "" {
				statusBucket[state]++
			}
			if muted, _ := m["muted"].(bool); muted {
				mutedBucket["true"]++
			} else {
				mutedBucket["false"]++
			}
		}
	}

	writeJSON(w, 200, map[string]any{
		"monitors": results,
		"counts": map[string]any{
			"status": bucketCounts(statusBucket),
			"muted":  bucketCounts(mutedBucket),
		},
		"metadata": map[string]any{
			"total":         len(results),
			"page":          0,
			"per_page":      30,
			"page_count":    1,
			"total_results": len(results),
		},
	})
}

// bucketCounts converts a tally map into Datadog's [{name, count}] envelope
// shape used inside `counts.{status,muted,tag,type,...}` on the monitor
// search response. Extracted so additional buckets (priority, type) can be
// added without duplicating the projection loop.
func bucketCounts(b map[string]int) []map[string]any {
	out := make([]map[string]any, 0, len(b))
	for name, count := range b {
		out = append(out, map[string]any{"name": name, "count": count})
	}
	return out
}

func (s *server) handleMonitorByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/monitor/")
	if path == "search" {
		s.handleMonitorSearch(w, r)
		return
	}

	// Verb before ID: a wrong method on a bad ID should still read as 405,
	// matching every other by-ID handler in the package.
	if !requireMethod(w, r, http.MethodGet) {
		return
	}

	parts := strings.Split(path, "/")
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		writeError(w, 400, "invalid monitor ID")
		return
	}

	m, ok := s.findMonitor(id)
	if !ok {
		writeError(w, 404, "Monitor not found")
		return
	}
	writeJSON(w, 200, m)
}
