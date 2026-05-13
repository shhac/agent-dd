package mockdd

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"time"
)

func handleTraceSearch(w http.ResponseWriter, r *http.Request) {
	// Mirror the real /api/v2/spans/events/search contract: reject any
	// body that is missing the JSON:API data envelope.
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	if _, hasData := body["data"]; !hasData {
		writeJSON(w, 400, map[string]any{
			"errors": []string{`document is missing required top-level members; must have one of: "data", "meta", "errors"`},
		})
		return
	}

	traceServices := []string{"checkout-service", "payment-api", "search-service", "gateway-api"}
	resources := []string{"POST /api/v1/orders", "GET /api/v1/search", "POST /api/v1/payments", "GET /api/v1/health"}
	errorTypes := []map[string]string{
		{"message": "connection refused", "type": "NetworkError"},
		{"message": "deadline exceeded", "type": "TimeoutError"},
		{"message": "row not found", "type": "DBError"},
	}

	data := make([]map[string]any, 10)
	for i := range 10 {
		svc := traceServices[i%len(traceServices)]
		start := time.Now().Add(-time.Duration(i) * time.Second)
		end := start.Add(time.Duration(rand.Intn(2_000)) * time.Millisecond)
		attrs := map[string]any{
			"trace_id":        fmt.Sprintf("trace-%08x", rand.Intn(math.MaxInt32)),
			"span_id":         fmt.Sprintf("span-%08x", rand.Intn(math.MaxInt32)),
			"service":         svc,
			"operation_name":  "http.request",
			"resource_name":   resources[i%len(resources)],
			"start_timestamp": start.UTC().Format(time.RFC3339Nano),
			"end_timestamp":   end.UTC().Format(time.RFC3339Nano),
			"env":             "prod",
			"tags":            []string{"team:checkout", "region:us-east-1"},
			"status":          "ok",
		}
		if i%4 == 0 {
			attrs["status"] = "error"
			attrs["error"] = errorTypes[i%len(errorTypes)]
		}
		data[i] = map[string]any{
			"type":       "spans",
			"attributes": attrs,
		}
	}

	writeJSON(w, 200, map[string]any{
		"data": data,
		"meta": map[string]any{"page": map[string]any{"after": ""}},
	})
}

func handleServiceList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"data": map[string]any{
			"attributes": map[string]any{
				"services": services,
			},
			"id":   "1",
			"type": "services_list",
		},
	})
}
