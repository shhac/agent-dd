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

	data := make([]map[string]any, 10)
	for i := range 10 {
		svc := traceServices[i%len(traceServices)]
		status := "ok"
		errVal := 0
		if i%4 == 0 {
			status = "error"
			errVal = 1
		}
		data[i] = map[string]any{
			"type": "spans",
			"attributes": map[string]any{
				"trace_id": fmt.Sprintf("trace-%08x", rand.Intn(math.MaxInt32)),
				"span_id":  fmt.Sprintf("span-%08x", rand.Intn(math.MaxInt32)),
				"service":  svc,
				"name":     "http.request",
				"resource": resources[i%len(resources)],
				"duration": float64(rand.Intn(2000000000)),
				"status":   status,
				"error":    errVal,
				"start":    time.Now().Add(-time.Duration(i) * time.Second).UnixNano(),
			},
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
