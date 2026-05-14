package mockdd

import (
	"net/http"
	"strconv"
	"strings"
)

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
	statusBucket := map[string]int{}
	mutedBucket := map[string]int{}
	for _, m := range monitors {
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

func handleMonitorByID(w http.ResponseWriter, r *http.Request) {
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
