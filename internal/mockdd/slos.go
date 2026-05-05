package mockdd

import (
	"net/http"
	"strings"
)

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
