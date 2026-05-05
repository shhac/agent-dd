package mockdd

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strings"
)

func handleIncidents(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
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
