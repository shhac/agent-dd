package mockdd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

var (
	activeDowntimes = make([]map[string]any, 0)
	downtimeCounter int
)

func handleDowntimes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)

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
