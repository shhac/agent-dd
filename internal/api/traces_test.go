package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shhac/agent-dd/internal/api"
)

func TestSearchTraces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/spans/events/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		// Real API requires the JSON:API envelope.
		data, ok := body["data"].(map[string]any)
		if !ok {
			t.Fatal("missing data envelope in request body")
		}
		if dt, _ := data["type"].(string); dt != "search_request" {
			t.Errorf("expected data.type=search_request, got %q", dt)
		}
		attrs, ok := data["attributes"].(map[string]any)
		if !ok {
			t.Fatal("missing data.attributes in request body")
		}
		filter, ok := attrs["filter"].(map[string]any)
		if !ok {
			t.Fatal("missing filter in attributes")
		}
		query, _ := filter["query"].(string)
		if query != "service:web-api status:error" {
			t.Errorf("expected query 'service:web-api status:error', got %q", query)
		}
		page, _ := attrs["page"].(map[string]any)
		if limit, _ := page["limit"].(float64); int(limit) != 10 {
			t.Errorf("expected page.limit=10, got %v", page["limit"])
		}
		if sort, _ := attrs["sort"].(string); sort != "-timestamp" {
			t.Errorf("expected sort=-timestamp, got %q", sort)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"type": "spans",
					"attributes": map[string]any{
						"trace_id": "trace-id-1",
						"service":  "web-api",
						"name":     "http.request",
						"duration": 1.5,
						"status":   "error",
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	resp, err := client.SearchTraces(context.Background(), "status:error", "web-api", "2024-01-15T09:00:00Z", "2024-01-15T10:00:00Z", 10)
	if err != nil {
		t.Fatalf("SearchTraces failed: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 span, got %d", len(resp.Data))
	}
	if resp.Data[0].Attributes.TraceID != "trace-id-1" {
		t.Errorf("expected trace_id=trace-id-1, got %s", resp.Data[0].Attributes.TraceID)
	}
	if resp.Data[0].Attributes.Service != "web-api" {
		t.Errorf("expected service=web-api, got %s", resp.Data[0].Attributes.Service)
	}
}

// Regression for the JSON:API 400 the spans-search endpoint returns when
// the body is sent without the data envelope. Mirrors the validation the
// real API performs.
func TestSearchTracesRejectsFlatBody(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		// Simulate the real Datadog 400: must have data/meta/errors.
		if _, hasData := body["data"]; !hasData {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errors":["document is missing required top-level members; must have one of: \"data\", \"meta\", \"errors\""]}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	if _, err := client.SearchTraces(context.Background(), "*", "svc", "now-1h", "now", 5); err != nil {
		t.Fatalf("SearchTraces should succeed against envelope-checking server: %v", err)
	}
	if !called {
		t.Fatal("server handler was not invoked")
	}
}

func TestSearchTracesCursor(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"type": "spans",
					"attributes": map[string]any{
						"trace_id": "trace-id-2",
						"service":  "web-api",
					},
				},
			},
			"meta": map[string]any{
				"page": map[string]any{
					"after": "next-page-cursor",
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	resp, err := client.SearchTraces(context.Background(), "*", "", "2024-01-15T09:00:00Z", "2024-01-15T10:00:00Z", 10)
	if err != nil {
		t.Fatalf("SearchTraces failed: %v", err)
	}
	if resp.Cursor() != "next-page-cursor" {
		t.Errorf("expected cursor 'next-page-cursor', got %q", resp.Cursor())
	}
}

// Regression: the v2 spans events API returns error as an object, not a legacy
// int flag. A plain `int` field caused an unmarshal panic on real error spans.
func TestSearchTracesErrorObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"type": "spans",
					"attributes": map[string]any{
						"trace_id": "trace-id-err",
						"service":  "bookingservice",
						"status":   "error",
						"error": map[string]any{
							"message": "connection refused",
							"type":    "NetworkError",
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	resp, err := client.SearchTraces(context.Background(), "status:error", "bookingservice", "now-1h", "now", 10)
	if err != nil {
		t.Fatalf("SearchTraces failed: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 span, got %d", len(resp.Data))
	}
	e := resp.Data[0].Attributes.Error
	if e == nil {
		t.Fatal("expected non-nil error")
	}
	if e.Message != "connection refused" {
		t.Errorf("expected message 'connection refused', got %q", e.Message)
	}
	if e.Type != "NetworkError" {
		t.Errorf("expected type 'NetworkError', got %q", e.Type)
	}
}

func TestListServices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v2/apm/services" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if env := r.URL.Query().Get("filter[env]"); env != "production" {
			t.Errorf("expected filter[env]=production, got %q", env)
		}

		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"attributes": map[string]any{
					"services": []string{"web-api", "auth-svc", "worker-job"},
				},
				"id":   "1",
				"type": "services_list",
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	services, err := client.ListServices(context.Background(), "production", "")
	if err != nil {
		t.Fatalf("ListServices failed: %v", err)
	}
	if len(services) != 3 {
		t.Fatalf("expected 3 services, got %d", len(services))
	}

	names := make(map[string]bool)
	for _, s := range services {
		names[s.Name] = true
	}
	for _, expected := range []string{"web-api", "auth-svc", "worker-job"} {
		if !names[expected] {
			t.Errorf("expected service %q not found", expected)
		}
	}
}

// Empty services slice should yield an empty result, not nil-deref.
func TestListServicesEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"attributes": map[string]any{
					"services": []string{},
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	services, err := client.ListServices(context.Background(), "production", "")
	if err != nil {
		t.Fatalf("ListServices empty: %v", err)
	}
	if len(services) != 0 {
		t.Errorf("expected 0 services, got %d", len(services))
	}
}

func TestListServicesSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"attributes": map[string]any{
					"services": []string{"web-api", "web-frontend", "auth-svc"},
				},
			},
		})
	}))
	defer srv.Close()

	client := api.NewTestClient(srv.URL+"/api", "key", "app")
	services, err := client.ListServices(context.Background(), "*", "web")
	if err != nil {
		t.Fatalf("ListServices failed: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services matching 'web', got %d", len(services))
	}
}
