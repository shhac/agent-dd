package mockdd

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strings"
)

func (s *server) handleIncidents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.createIncident(w, r)
	case http.MethodGet:
		s.listIncidents(w, r)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// Incident writes deliberately do not persist to the store: they echo a
// synthetic response and leave s.incidents alone, so a created incident will
// not appear in a subsequent list. Nothing needs the round trip yet. If that
// changes, model it on the downtime handlers — which do persist — rather than
// on this one.
func (s *server) createIncident(w http.ResponseWriter, r *http.Request) {
	attrs := decodeAttributes(r)
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
}

func (s *server) listIncidents(w http.ResponseWriter, r *http.Request) {
	stateFilter := r.URL.Query().Get("filter[state]")
	results := make([]map[string]any, 0)
	for _, inc := range s.allIncidents() {
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

func (s *server) handleIncidentByID(w http.ResponseWriter, r *http.Request) {
	incID := strings.TrimPrefix(r.URL.Path, "/api/v2/incidents/")

	switch r.Method {
	case http.MethodPatch:
		s.updateIncident(w, r, incID)
	case http.MethodGet:
		s.getIncident(w, r, incID)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPatch)
	}
}

func (s *server) updateIncident(w http.ResponseWriter, r *http.Request, incID string) {
	attrs := decodeAttributes(r)
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
}

func (s *server) getIncident(w http.ResponseWriter, r *http.Request, incID string) {
	for _, inc := range s.allIncidents() {
		if id, _ := inc["id"].(string); id == incID {
			resp := map[string]any{"data": inc}
			if include := r.URL.Query().Get("include"); strings.Contains(include, "commander_user") {
				resp["included"] = []map[string]any{incidentCommander}
			}
			writeJSON(w, 200, resp)
			return
		}
	}
	writeError(w, 404, "Incident not found")
}
