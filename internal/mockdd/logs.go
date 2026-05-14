package mockdd

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

func handleLogSearch(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)

	filter, _ := body["filter"].(map[string]any)
	query, _ := filter["query"].(string)

	page, _ := body["page"].(map[string]any)
	limit := 50
	if l, ok := page["limit"].(float64); ok {
		limit = int(l)
	}

	entries := make([]map[string]any, 0, limit)
	for i := range limit {
		msg := logMessages[i%len(logMessages)]

		// `*` is Datadog's match-all sentinel and must return every entry —
		// treating it as a literal substring (zero matches) diverges from
		// the real API and breaks integration tests that use it.
		if query != "" && query != "*" {
			q := strings.ToLower(query)
			match := strings.Contains(strings.ToLower(msg.Service), q) ||
				strings.Contains(strings.ToLower(msg.Status), q) ||
				strings.Contains(strings.ToLower(msg.Message), q) ||
				strings.Contains(q, "service:"+strings.ToLower(msg.Service)) ||
				strings.Contains(q, "status:"+strings.ToLower(msg.Status))
			if !match {
				continue
			}
		}

		entries = append(entries, map[string]any{
			"id":   fmt.Sprintf("log-%08d", rand.Intn(99999999)),
			"type": "log",
			"attributes": map[string]any{
				"timestamp": time.Now().Add(-time.Duration(i) * time.Minute).Format(time.RFC3339),
				"service":   msg.Service,
				"status":    msg.Status,
				"message":   msg.Message,
				"host":      hosts[i%len(hosts)]["name"],
			},
		})
		if len(entries) >= limit {
			break
		}
	}

	writeJSON(w, 200, map[string]any{
		"data": entries,
		"meta": map[string]any{"page": map[string]any{"after": ""}},
	})
}

func handleLogAggregate(w http.ResponseWriter, _ *http.Request) {
	buckets := []map[string]any{
		{"by": map[string]any{"service": "checkout-service", "status": "error"}, "computes": map[string]any{"c0": 142}},
		{"by": map[string]any{"service": "search-service", "status": "warn"}, "computes": map[string]any{"c0": 87}},
		{"by": map[string]any{"service": "payment-api", "status": "error"}, "computes": map[string]any{"c0": 23}},
		{"by": map[string]any{"service": "gateway-api", "status": "info"}, "computes": map[string]any{"c0": 5420}},
		{"by": map[string]any{"service": "inventory-worker", "status": "error"}, "computes": map[string]any{"c0": 56}},
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"buckets": buckets}})
}
