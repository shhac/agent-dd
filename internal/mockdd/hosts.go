package mockdd

import (
	"maps"
	"math/rand"
	"net/http"
	"strings"
)

func (s *server) handleHosts(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	filter := r.URL.Query().Get("filter")
	now := nowUnix()

	result := make([]map[string]any, 0)
	for _, h := range hosts {
		name, _ := h["name"].(string)
		if filter != "" && !strings.Contains(name, filter) {
			continue
		}
		host := maps.Clone(h)
		host["last_reported_time"] = now - int64(rand.Intn(300))
		result = append(result, host)
	}
	writeJSON(w, 200, map[string]any{
		"host_list":      result,
		"total_returned": len(result),
		"total_matching": len(result),
	})
}

func (s *server) handleHostMute(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	writeJSON(w, 200, map[string]any{"action": "Muted", "hostname": "mocked"})
}
