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
		severity, _ := attrs["severity"].(string)
		if severity == "" {
			severity = "SEV-3"
		}
		impacted, _ := attrs["customer_impacted"].(bool)
		inc := map[string]any{
			"id":   fmt.Sprintf("inc-%08x", rand.Intn(math.MaxInt32)),
			"type": "incidents",
			"attributes": map[string]any{
				"title":             attrs["title"],
				"state":             "active",
				"severity":          severity,
				"customer_impacted": impacted,
				"created":           nowRFC3339(),
			},
		}
		writeJSON(w, 201, map[string]any{"data": inc})
		return
	}

	stateFilter := r.URL.Query().Get("filter[state]")
	results := make([]map[string]any, 0)
	for _, inc := range incidents {
		if stateFilter != "" {
			attrs, _ := inc["attributes"].(map[string]any)
			if state, _ := attrs["state"].(string); state != stateFilter {
				continue
			}
		}
		results = append(results, inc)
	}
	resp := map[string]any{"data": results}
	if include := r.URL.Query().Get("include"); strings.Contains(include, "commander_user") {
		resp["included"] = []map[string]any{incidentCommander}
	}
	writeJSON(w, 200, resp)
}

func handleIncidentByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v2/incidents/")
	incID := path

	if r.Method == http.MethodPatch {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		data, _ := body["data"].(map[string]any)
		attrs, _ := data["attributes"].(map[string]any)
		fields, _ := attrs["fields"].(map[string]any)
		stateField, _ := fields["state"].(map[string]any)
		newState, _ := stateField["value"].(string)
		if newState == "" {
			newState = "stable"
		}
		severity, _ := attrs["severity"].(string)
		if severity == "" {
			severity = "SEV-3"
		}
		writeJSON(w, 200, map[string]any{
			"data": map[string]any{
				"id":   incID,
				"type": "incidents",
				"attributes": map[string]any{
					"title":    "Updated incident",
					"state":    newState,
					"severity": severity,
				},
			},
		})
		return
	}

	for _, inc := range incidents {
		if id, _ := inc["id"].(string); id == incID {
			resp := map[string]any{"data": inc}
			if include := r.URL.Query().Get("include"); strings.Contains(include, "commander_user") {
				resp["included"] = []map[string]any{incidentCommander}
			}
			writeJSON(w, 200, resp)
			return
		}
	}
	writeJSON(w, 404, map[string]any{"errors": []string{"Incident not found"}})
}
