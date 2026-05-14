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

	statusCounts := make([]map[string]any, 0, len(statusBucket))
	for name, c := range statusBucket {
		statusCounts = append(statusCounts, map[string]any{"name": name, "count": c})
	}
	mutedCounts := make([]map[string]any, 0, len(mutedBucket))
	for name, c := range mutedBucket {
		mutedCounts = append(mutedCounts, map[string]any{"name": name, "count": c})
	}

	writeJSON(w, 200, map[string]any{
		"monitors": results,
		"counts": map[string]any{
			"status": statusCounts,
			"muted":  mutedCounts,
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
