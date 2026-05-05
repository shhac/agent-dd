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
	for _, m := range monitors {
		name, _ := m["name"].(string)
		if query == "" || strings.Contains(strings.ToLower(name), strings.ToLower(query)) {
			results = append(results, m)
		}
	}
	writeJSON(w, 200, map[string]any{"monitors": results})
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
