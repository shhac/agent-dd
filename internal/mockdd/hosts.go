package mockdd

import (
	"math/rand"
	"net/http"
	"strings"
)

func handleHosts(w http.ResponseWriter, r *http.Request) {
	filter := r.URL.Query().Get("filter")
	now := nowUnix()

	result := make([]map[string]any, 0)
	for _, h := range hosts {
		copy := make(map[string]any)
		for k, v := range h {
			copy[k] = v
		}
		copy["last_reported_time"] = now - int64(rand.Intn(300))

		name, _ := h["name"].(string)
		if filter != "" && !strings.Contains(name, filter) {
			continue
		}
		result = append(result, copy)
	}
	writeJSON(w, 200, map[string]any{
		"host_list":      result,
		"total_returned": len(result),
		"total_matching": len(result),
	})
}

func handleHostMute(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"action": "Muted", "hostname": "mocked"})
}
