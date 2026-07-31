package mockdd

import (
	"maps"
	"net/http"
	"strconv"
	"strings"
)

func (s *server) handleEventList(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	now := nowUnix()
	result := make([]map[string]any, len(events))
	for i, e := range events {
		event := maps.Clone(e)
		event["date_happened"] = now - int64((len(events)-i)*600)
		result[i] = event
	}
	writeJSON(w, 200, map[string]any{"events": result})
}

func (s *server) handleEventByID(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/events/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, 400, "invalid event ID")
		return
	}
	for _, e := range events {
		if eid, _ := e["id"].(int); int64(eid) == id {
			writeJSON(w, 200, map[string]any{"event": e})
			return
		}
	}
	writeError(w, 404, "Event not found")
}
