package mockdd

import (
	"net/http"
	"strconv"
	"strings"
)

func handleEventList(w http.ResponseWriter, _ *http.Request) {
	now := nowUnix()
	result := make([]map[string]any, len(events))
	for i, e := range events {
		copy := make(map[string]any)
		for k, v := range e {
			copy[k] = v
		}
		copy["date_happened"] = now - int64((len(events)-i)*600)
		result[i] = copy
	}
	writeJSON(w, 200, map[string]any{"events": result})
}

func handleEventByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/events/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, 400, map[string]any{"errors": []string{"invalid event ID"}})
		return
	}
	for _, e := range events {
		if eid, _ := e["id"].(int); int64(eid) == id {
			writeJSON(w, 200, map[string]any{"event": e})
			return
		}
	}
	writeJSON(w, 404, map[string]any{"errors": []string{"Event not found"}})
}
