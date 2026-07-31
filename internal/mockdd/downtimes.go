package mockdd

import (
	"fmt"
	"net/http"
	"strings"
)

func downtimeID(counter int) string {
	return fmt.Sprintf("dt-%06d", counter)
}

func (s *server) handleDowntimes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.createDowntime(w, r)
	case http.MethodGet:
		s.listDowntimes(w, r)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *server) createDowntime(w http.ResponseWriter, r *http.Request) {
	attrs := decodeAttributes(r)

	dt := s.addDowntime(map[string]any{
		"type": "downtime",
		"attributes": map[string]any{
			"status":  "active",
			"message": attrs["message"],
			"scope":   attrs["scope"],
		},
	})
	writeJSON(w, 200, map[string]any{"data": dt})
}

func (s *server) listDowntimes(w http.ResponseWriter, r *http.Request) {
	monitorFilter := r.URL.Query().Get("filter[monitor_id]")
	statusFilter := r.URL.Query().Get("filter[status]")

	results := make([]map[string]any, 0)
	for _, dt := range s.allDowntimes() {
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

func (s *server) handleDowntimeByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w, http.MethodDelete)
		return
	}

	dtID := strings.TrimPrefix(r.URL.Path, "/api/v2/downtime/")
	if !s.removeDowntime(dtID) {
		writeError(w, 404, "Downtime not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
